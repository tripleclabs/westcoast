package actor

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"westcoast/src/internal/metrics"
)

type RuntimeOption func(*Runtime)

type ActorOption func(*actorConfig)

type actorConfig struct {
	mailboxCapacity int
}

type actorInstance struct {
	id           string
	status       atomic.Int32
	mailbox      *Mailbox
	handler      Handler
	state        actorState
	initialState any
	restarts     int
	cancel       context.CancelFunc
	cfg          actorConfig
	mu           sync.RWMutex
}

type Runtime struct {
	registry *actorRegistry
	emitter  EventEmitter
	policy   SupervisorPolicy
	metrics  metrics.Hooks
	routing  *typeRoutingRegistry
	now      func() time.Time
	msgSeq   atomic.Uint64
	eventSeq atomic.Uint64
	outcomes *outcomeStore
	resolver PIDResolver
	actorPID sync.Map // actorID -> PID
}

func WithEmitter(e EventEmitter) RuntimeOption {
	return func(r *Runtime) { r.emitter = e }
}

func WithSupervisor(p SupervisorPolicy) RuntimeOption {
	return func(r *Runtime) { r.policy = p }
}

func WithMetrics(h metrics.Hooks) RuntimeOption {
	return func(r *Runtime) { r.metrics = h }
}

func NewRuntime(opts ...RuntimeOption) *Runtime {
	r := &Runtime{
		registry: newRegistry(),
		emitter:  NopEmitter{},
		policy:   DefaultSupervisor{MaxRestarts: 1},
		metrics:  metrics.NopHooks{},
		routing:  newTypeRoutingRegistry(),
		now:      time.Now,
		outcomes: newOutcomeStore(),
		resolver: NewInMemoryPIDResolver(),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func WithMailboxCapacity(capacity int) ActorOption {
	return func(c *actorConfig) { c.mailboxCapacity = capacity }
}

func (r *Runtime) CreateActor(id string, initialState any, handler Handler, opts ...ActorOption) (*ActorRef, error) {
	if id == "" {
		return nil, fmt.Errorf("empty actor id")
	}
	if handler == nil {
		return nil, fmt.Errorf("nil handler")
	}
		cfg := actorConfig{mailboxCapacity: 1024}
	for _, opt := range opts {
		opt(&cfg)
	}
	ctx, cancel := context.WithCancel(context.Background())
	inst := &actorInstance{
		id:           id,
		mailbox:      NewMailbox(cfg.mailboxCapacity),
		handler:      handler,
		state:        newActorState(initialState),
		initialState: initialState,
		cancel:       cancel,
		cfg:          cfg,
	}
	inst.status.Store(int32(ActorStartingCode))
	if err := r.registry.put(id, inst); err != nil {
		cancel()
		return nil, err
	}
	go r.runActor(ctx, inst)
	return &ActorRef{runtime: r, actorID: id}, nil
}

type actorStatusCode int32

const (
	ActorStartingCode actorStatusCode = 0
	ActorRunningCode  actorStatusCode = 1
	ActorRestartCode  actorStatusCode = 2
	ActorStoppedCode  actorStatusCode = 3
)

func (r *Runtime) emit(eventType EventType, actorID string, messageID uint64, result, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:   eid,
		Type:      eventType,
		ActorID:   actorID,
		MessageID: messageID,
		Timestamp: r.now(),
		Result:    result,
		ErrorCode: code,
	})
}

func (r *Runtime) emitLocal(eventType EventType, actorID string, msg Message, result, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:       eid,
		Type:          eventType,
		ActorID:       actorID,
		MessageID:     msg.ID,
		TypeName:      msg.TypeName,
		SchemaVersion: msg.SchemaVersion,
		Timestamp:     r.now(),
		Result:        result,
		ErrorCode:     code,
	})
}

func (r *Runtime) emitPID(eventType EventType, pid PID, messageID uint64, outcome PIDDeliveryOutcome, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:       eid,
		Type:          eventType,
		ActorID:       pid.ActorID,
		MessageID:     messageID,
		PIDNamespace:  pid.Namespace,
		PIDActorID:    pid.ActorID,
		PIDGeneration: pid.Generation,
		Timestamp:     r.now(),
		Result:        string(outcome),
		ErrorCode:     code,
	})
}

func (r *Runtime) runActor(ctx context.Context, inst *actorInstance) {
	inst.status.Store(int32(ActorRunningCode))
	r.emit(EventActorStarted, inst.id, 0, string(ActorRunning), "")
	for {
		select {
		case <-ctx.Done():
			inst.status.Store(int32(ActorStoppedCode))
			r.emit(EventActorStopped, inst.id, 0, string(ActorStopped), "")
			return
		case <-inst.mailbox.Notify():
			for {
				msg, ok := inst.mailbox.Dequeue()
				if !ok {
					break
				}
				r.processMessage(inst, msg)
			}
		}
	}
}

func (r *Runtime) processMessage(inst *actorInstance, msg Message) {
	started := r.now()
	defer func() {
		r.metrics.ObserveProcessingLatency(inst.id, r.now().Sub(started))
	}()

	var (
		next any
		err  error
	)
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("panic: %v", rec)
			}
		}()
		inst.mu.RLock()
		cur := inst.state.value()
		inst.mu.RUnlock()
		next, err = inst.handler(context.Background(), cur, msg)
	}()

	if err != nil {
		r.outcomes.put(ProcessingOutcome{MessageID: msg.ID, ActorID: inst.id, Result: ResultFailed, CompletedAt: r.now(), ErrorCode: "handler_error"})
		r.emit(EventActorFailed, inst.id, msg.ID, string(ResultFailed), "handler_error")
		decision := r.policy.Decide(inst.id, err, inst.restarts)
		switch decision {
		case DecisionRestart:
			inst.status.Store(int32(ActorRestartCode))
			inst.restarts++
			r.metrics.ObserveRestart(inst.id)
			inst.mu.Lock()
			inst.state = newActorState(inst.initialState)
			inst.mu.Unlock()
			if v, ok := r.actorPID.Load(inst.id); ok {
				oldPID := v.(PID)
				r.resolver.SetState(oldPID, PIDRouteRestarting)
				if newPID, bumped := r.resolver.BumpGeneration(oldPID); bumped {
					r.actorPID.Store(inst.id, newPID)
				}
			}
			inst.status.Store(int32(ActorRunningCode))
			r.emit(EventActorRestarted, inst.id, msg.ID, string(ActorRunning), "")
		default:
			inst.cancel()
			inst.mailbox.Close()
		}
		return
	}

	inst.mu.Lock()
	inst.state.apply(next)
	inst.mu.Unlock()
	r.outcomes.put(ProcessingOutcome{MessageID: msg.ID, ActorID: inst.id, Result: ResultDelivered, CompletedAt: r.now()})
	r.emitLocal(EventMessageProcessed, inst.id, msg, string(ResultDelivered), "")
}

func (r *Runtime) Send(_ context.Context, actorID string, payload any) SubmitAck {
	sendStart := r.now()
	defer func() {
		r.metrics.ObserveLocalSendLatency(actorID, r.now().Sub(sendStart))
	}()

	inst, ok := r.registry.get(actorID)
	msgID := r.msgSeq.Add(1)
	if !ok {
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedFound, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedFound)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedFound), string(SubmitRejectedFound))
		return SubmitAck{Result: SubmitRejectedFound, MessageID: msgID}
	}

	if actorStatusCode(inst.status.Load()) == ActorStoppedCode {
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedStop, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedStop)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedStop), string(SubmitRejectedStop))
		return SubmitAck{Result: SubmitRejectedStop, MessageID: msgID}
	}

	if payload == nil {
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedNilPayload, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedNilPayload)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedNilPayload), string(SubmitRejectedNilPayload))
		r.metrics.ObserveLocalRouting(actorID, string(SubmitRejectedNilPayload))
		return SubmitAck{Result: SubmitRejectedNilPayload, MessageID: msgID}
	}

	typeName := messageTypeName(payload)
	schemaVersion := messageSchemaVersion(payload)
	msg := Message{
		ID:            msgID,
		ActorID:       actorID,
		Payload:       payload,
		TypeName:      typeName,
		SchemaVersion: schemaVersion,
		AcceptedAt:    r.now(),
		Attempt:       1,
	}

	resolution := r.routing.resolve(actorID, typeName, schemaVersion)
	switch resolution.match {
	case routeExact:
		r.metrics.ObserveLocalRouting(actorID, string(EventMessageRoutedExact))
		r.emitLocal(EventMessageRoutedExact, actorID, msg, string(SubmitAccepted), "")
	case routeFallback:
		r.metrics.ObserveLocalRouting(actorID, string(EventMessageRoutedFallback))
		r.emitLocal(EventMessageRoutedFallback, actorID, msg, string(SubmitAccepted), "")
	case routeVersionMismatch:
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedVersionMismatch, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedVersionMismatch)})
		r.emitLocal(EventMessageRejected, actorID, msg, string(ResultRejectedVersionMismatch), string(SubmitRejectedVersionMismatch))
		r.metrics.ObserveLocalRouting(actorID, string(SubmitRejectedVersionMismatch))
		return SubmitAck{Result: SubmitRejectedVersionMismatch, MessageID: msgID}
	case routeUnsupportedType:
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedUnsupportedType, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedUnsupportedType)})
		r.emitLocal(EventMessageRejected, actorID, msg, string(ResultRejectedUnsupportedType), string(SubmitRejectedUnsupportedType))
		r.metrics.ObserveLocalRouting(actorID, string(SubmitRejectedUnsupportedType))
		return SubmitAck{Result: SubmitRejectedUnsupportedType, MessageID: msgID}
	case routeNoRules:
		// Backward-compatible mode when no explicit route rules were registered.
	}

	start := r.now()
	res := inst.mailbox.Enqueue(msg)
	r.metrics.ObserveEnqueueLatency(actorID, r.now().Sub(start))
	r.metrics.ObserveMailboxDepth(actorID, inst.mailbox.Depth())
	if res == SubmitRejectedFull {
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedFull, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedFull)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedFull), string(SubmitRejectedFull))
	}
	if res == SubmitRejectedStop {
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedStop, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedStop)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedStop), string(SubmitRejectedStop))
	}
	return SubmitAck{Result: res, MessageID: msgID}
}

func (r *Runtime) RegisterTypeRoute(actorID, typeName, schemaVersion, handlerKey string) error {
	if _, ok := r.registry.get(actorID); !ok {
		return ErrActorNotFound
	}
	r.routing.registerExact(actorID, typeName, schemaVersion, handlerKey)
	return nil
}

func (r *Runtime) RegisterFallbackRoute(actorID, handlerKey string) error {
	if _, ok := r.registry.get(actorID); !ok {
		return ErrActorNotFound
	}
	r.routing.registerFallback(actorID, handlerKey)
	return nil
}

func (r *Runtime) Stop(actorID string) StopResult {
	inst, ok := r.registry.get(actorID)
	if !ok {
		return StopNotFound
	}
	if r.Status(actorID) == ActorStopped {
		return StopAlready
	}
	inst.status.Store(int32(ActorStoppedCode))
	if v, ok := r.actorPID.Load(actorID); ok {
		r.resolver.SetState(v.(PID), PIDRouteStopped)
	}
	inst.cancel()
	inst.mailbox.Close()
	return StopStopped
}

func (r *Runtime) Status(actorID string) ActorStatus {
	inst, ok := r.registry.get(actorID)
	if !ok {
		return ActorStopped
	}
	s := actorStatusCode(inst.status.Load())
	switch s {
	case ActorStartingCode:
		return ActorStarting
	case ActorRunningCode:
		return ActorRunning
	case ActorRestartCode:
		return ActorRestarting
	default:
		return ActorStopped
	}
}

func (r *Runtime) ActorRef(actorID string) (*ActorRef, error) {
	if _, ok := r.registry.get(actorID); !ok {
		return nil, ErrActorNotFound
	}
	return &ActorRef{runtime: r, actorID: actorID}, nil
}

func (r *Runtime) Outcome(messageID uint64) (ProcessingOutcome, bool) {
	return r.outcomes.get(messageID)
}

func (r *Runtime) IssuePID(namespace, actorID string) (PID, error) {
	if _, ok := r.registry.get(actorID); !ok {
		return PID{}, ErrActorNotFound
	}
	pid := PID{Namespace: namespace, ActorID: actorID, Generation: 1}
	if err := pid.Validate(); err != nil {
		return PID{}, err
	}
	r.resolver.Register(pid)
	r.actorPID.Store(actorID, pid)
	return pid, nil
}

func (r *Runtime) PIDForActor(actorID string) (PID, bool) {
	v, ok := r.actorPID.Load(actorID)
	if !ok {
		return PID{}, false
	}
	return v.(PID), true
}

func (r *Runtime) ResolvePID(pid PID) (PIDResolverEntry, bool) {
	start := r.now()
	entry, ok := r.resolver.Resolve(pid)
	r.metrics.ObservePIDLookupLatency(pid.Key(), r.now().Sub(start))
	if !ok {
		r.emitPID(EventPIDUnresolved, pid, 0, PIDUnresolved, string(PIDUnresolved))
		return PIDResolverEntry{}, false
	}
	r.emitPID(EventPIDResolved, pid, 0, PIDDelivered, "")
	return entry, true
}

func (r *Runtime) SendPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	start := r.now()
	entry, ok := r.resolver.Resolve(pid)
	r.metrics.ObservePIDLookupLatency(pid.Key(), r.now().Sub(start))
	msgID := r.msgSeq.Add(1)
	if !ok {
		if _, exists := r.registry.get(pid.ActorID); !exists {
			r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedNotFound, string(PIDRejectedNotFound))
			return PIDSendAck{Outcome: PIDRejectedNotFound, MessageID: msgID}
		}
		r.emitPID(EventPIDUnresolved, pid, msgID, PIDUnresolved, string(PIDUnresolved))
		return PIDSendAck{Outcome: PIDUnresolved, MessageID: msgID}
	}
	if entry.RouteState == PIDRouteStopped {
		r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedStopped, string(PIDRejectedStopped))
		return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: msgID}
	}
	if pid.Generation != entry.CurrentGeneration {
		r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedStaleGeneration, string(PIDRejectedStaleGeneration))
		return PIDSendAck{Outcome: PIDRejectedStaleGeneration, MessageID: msgID}
	}
	ack := r.Send(ctx, pid.ActorID, payload)
	if ack.Result != SubmitAccepted {
		r.emitPID(EventPIDRejected, pid, ack.MessageID, PIDRejectedStopped, string(PIDRejectedStopped))
		return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: ack.MessageID}
	}
	r.emitPID(EventPIDDelivered, pid, ack.MessageID, PIDDelivered, "")
	return PIDSendAck{Outcome: PIDDelivered, MessageID: ack.MessageID}
}
