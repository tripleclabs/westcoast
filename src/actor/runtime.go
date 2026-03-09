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
	startHook       LifecycleHook
	stopHook        LifecycleHook
	stopHookTimeout time.Duration
}

type actorInstance struct {
	id           string
	status       atomic.Int32
	mailbox      *Mailbox
	handler      Handler
	state        actorState
	initialState any
	restarts     int
	lastDecision SupervisionDecision
	ctx          context.Context
	cancel       context.CancelFunc
	cfg          actorConfig
	startHook    LifecycleHook
	stopHook     LifecycleHook
	mu           sync.RWMutex
}

type Runtime struct {
	registry  *actorRegistry
	names     *namedRegistry
	emitter   EventEmitter
	policy    SupervisorPolicy
	metrics   metrics.Hooks
	routing   *typeRoutingRegistry
	now       func() time.Time
	msgSeq    atomic.Uint64
	eventSeq  atomic.Uint64
	outcomes  *outcomeStore
	resolver  PIDResolver
	actorPID  sync.Map // actorID -> PID
	policyMu  sync.RWMutex
	pidPolicy PIDInteractionPolicyMode
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
		registry:  newRegistry(),
		names:     newNamedRegistry(time.Now),
		emitter:   NopEmitter{},
		policy:    DefaultSupervisor{MaxRestarts: 1},
		metrics:   metrics.NopHooks{},
		routing:   newTypeRoutingRegistry(),
		now:       time.Now,
		outcomes:  newOutcomeStore(),
		resolver:  NewInMemoryPIDResolver(),
		pidPolicy: PIDInteractionPolicyDisabled,
	}
	for _, opt := range opts {
		opt(r)
	}
	r.names.now = r.now
	return r
}

func WithMailboxCapacity(capacity int) ActorOption {
	return func(c *actorConfig) { c.mailboxCapacity = capacity }
}

func WithStartHook(h LifecycleHook) ActorOption {
	return func(c *actorConfig) { c.startHook = h }
}

func WithStopHook(h LifecycleHook) ActorOption {
	return func(c *actorConfig) { c.stopHook = h }
}

func WithStopHookTimeout(d time.Duration) ActorOption {
	return func(c *actorConfig) { c.stopHookTimeout = d }
}

func (r *Runtime) CreateActor(id string, initialState any, handler Handler, opts ...ActorOption) (*ActorRef, error) {
	if id == "" {
		return nil, fmt.Errorf("empty actor id")
	}
	if handler == nil {
		return nil, fmt.Errorf("nil handler")
	}
	cfg := actorConfig{mailboxCapacity: 1024, stopHookTimeout: 5 * time.Second}
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
		ctx:          ctx,
		cancel:       cancel,
		cfg:          cfg,
		startHook:    cfg.startHook,
		stopHook:     cfg.stopHook,
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
	ActorStoppingCode actorStatusCode = 3
	ActorStoppedCode  actorStatusCode = 4
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

func (r *Runtime) emitDecision(eventType EventType, actorID string, messageID uint64, decision SupervisionDecision, restartCount int, result, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:             eid,
		Type:                eventType,
		ActorID:             actorID,
		MessageID:           messageID,
		SupervisionDecision: string(decision),
		RestartCount:        restartCount,
		Timestamp:           r.now(),
		Result:              result,
		ErrorCode:           code,
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

func (r *Runtime) emitRegistry(eventType EventType, name, actorID string, pid PID, result RegistryOperationResult, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:       eid,
		Type:          eventType,
		ActorID:       actorID,
		PIDNamespace:  pid.Namespace,
		PIDActorID:    pid.ActorID,
		PIDGeneration: pid.Generation,
		RegistryName:  name,
		Timestamp:     r.now(),
		Result:        string(result),
		ErrorCode:     code,
	})
}

func (r *Runtime) emitLifecycle(actorID string, phase LifecycleHookPhase, result LifecycleHookResult, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:        eid,
		Type:           EventLifecycleHook,
		ActorID:        actorID,
		LifecyclePhase: string(phase),
		Timestamp:      r.now(),
		Result:         string(result),
		ErrorCode:      code,
	})
	r.metrics.ObserveLifecycleHook(string(phase), string(result))
}

func (r *Runtime) emitGuardrail(actorID string, mode GatewayRouteMode, outcome GuardrailOutcomeType, code string) {
	eid := r.eventSeq.Add(1)
	policyMode := r.PIDInteractionPolicy()
	r.emitter.Emit(Event{
		EventID:     eid,
		Type:        EventGuardrailDecision,
		ActorID:     actorID,
		GatewayMode: string(mode),
		Timestamp:   r.now(),
		Result:      string(outcome),
		ErrorCode:   code,
	})
	r.metrics.ObserveGuardrailDecision("guardrail", string(outcome))
	r.outcomes.putGuardrail(GuardrailOutcome{
		ActorID:     actorID,
		PolicyMode:  policyMode,
		GatewayMode: mode,
		Outcome:     outcome,
		ReasonCode:  code,
		At:          r.now(),
	})
}

func (r *Runtime) emitReadiness(scope ReadinessScope, result ReadinessResult, evidence string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:        eid,
		Type:           EventReadinessValidation,
		ReadinessScope: string(scope),
		Timestamp:      r.now(),
		Result:         string(result),
		ErrorCode:      evidence,
	})
	r.metrics.ObserveGuardrailDecision(string(scope), string(result))
	r.outcomes.putReadiness(ReadinessValidationRecord{
		Scope:       scope,
		Result:      result,
		CheckedAt:   r.now(),
		EvidenceRef: evidence,
	})
}

func (r *Runtime) runLifecycleHook(ctx context.Context, inst *actorInstance, phase LifecycleHookPhase, h LifecycleHook) error {
	if h == nil {
		return nil
	}
	code := ""
	var err error
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				err = fmt.Errorf("panic: %v", rec)
				code = "panic"
			}
		}()
		err = h(ctx, inst.id)
		if err != nil {
			code = "error"
		}
	}()
	if err != nil {
		out := LifecycleHookOutcome{
			ActorID:     inst.id,
			Phase:       phase,
			Result:      LifecycleStartFailed,
			CompletedAt: r.now(),
			ErrorCode:   code,
		}
		if phase == LifecyclePhaseStop {
			out.Result = LifecycleStopFailed
		}
		r.outcomes.putLifecycle(out)
		r.emitLifecycle(inst.id, phase, out.Result, code)
		return err
	}
	out := LifecycleHookOutcome{
		ActorID:     inst.id,
		Phase:       phase,
		Result:      LifecycleStartSuccess,
		CompletedAt: r.now(),
	}
	if phase == LifecyclePhaseStop {
		out.Result = LifecycleStopSuccess
	}
	r.outcomes.putLifecycle(out)
	r.emitLifecycle(inst.id, phase, out.Result, "")
	return nil
}

func (r *Runtime) runLifecycleStopHookWithTimeout(inst *actorInstance, timeout time.Duration) {
	if inst.stopHook == nil {
		return
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	hookCtx, cancel := context.WithTimeout(inst.ctx, timeout)
	defer cancel()

	type hookResult struct {
		code string
		err  error
	}
	done := make(chan hookResult, 1)
	go func() {
		code := ""
		var err error
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					err = fmt.Errorf("panic: %v", rec)
					code = "panic"
				}
			}()
			err = inst.stopHook(hookCtx, inst.id)
			if err != nil {
				code = "error"
			}
		}()
		done <- hookResult{code: code, err: err}
	}()

	select {
	case res := <-done:
		if res.err != nil {
			r.outcomes.putLifecycle(LifecycleHookOutcome{
				ActorID:     inst.id,
				Phase:       LifecyclePhaseStop,
				Result:      LifecycleStopFailed,
				CompletedAt: r.now(),
				ErrorCode:   res.code,
			})
			r.emitLifecycle(inst.id, LifecyclePhaseStop, LifecycleStopFailed, res.code)
			return
		}
		r.outcomes.putLifecycle(LifecycleHookOutcome{
			ActorID:     inst.id,
			Phase:       LifecyclePhaseStop,
			Result:      LifecycleStopSuccess,
			CompletedAt: r.now(),
		})
		r.emitLifecycle(inst.id, LifecyclePhaseStop, LifecycleStopSuccess, "")
	case <-hookCtx.Done():
		r.outcomes.putLifecycle(LifecycleHookOutcome{
			ActorID:     inst.id,
			Phase:       LifecyclePhaseStop,
			Result:      LifecycleStopFailed,
			CompletedAt: r.now(),
			ErrorCode:   "timeout",
		})
		r.emitLifecycle(inst.id, LifecyclePhaseStop, LifecycleStopFailed, "timeout")
	}
}

func (r *Runtime) runStopHookWithTimeout(inst *actorInstance, timeout time.Duration) {
	r.runLifecycleStopHookWithTimeout(inst, timeout)
}

func (r *Runtime) runActor(ctx context.Context, inst *actorInstance) {
	if err := r.runLifecycleHook(ctx, inst, LifecyclePhaseStart, inst.startHook); err != nil {
		inst.status.Store(int32(ActorStoppedCode))
		inst.mailbox.Close()
		inst.cancel()
		r.emit(EventActorStopped, inst.id, 0, string(ActorStopped), ErrLifecycleStartFailed.Error())
		return
	}
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
		next, err = inst.handler(inst.ctx, cur, msg)
	}()

	if err != nil {
		r.metrics.ObservePanicIntercept(inst.id)
		decision := r.policy.Decide(inst.id, err, inst.restarts)
		r.outcomes.put(ProcessingOutcome{
			MessageID:            msg.ID,
			ActorID:              inst.id,
			Result:               ResultFailed,
			SupervisionDecision:  decision,
			SupervisionIteration: inst.restarts,
			CompletedAt:          r.now(),
			ErrorCode:            "handler_error",
		})
		r.emitDecision(EventActorFailed, inst.id, msg.ID, decision, inst.restarts, string(ResultFailed), "handler_error")
		inst.lastDecision = decision
		switch decision {
		case DecisionRestart:
			inst.status.Store(int32(ActorRestartCode))
			inst.restarts++
			r.metrics.ObserveMailboxPreservedDepth(inst.id, inst.mailbox.Depth())
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
			if startErr := r.runLifecycleHook(inst.ctx, inst, LifecyclePhaseStart, inst.startHook); startErr != nil {
				inst.status.Store(int32(ActorStoppedCode))
				inst.cancel()
				inst.mailbox.Close()
				r.cleanupRegistryForActor(inst.id, RegistryUnregisterLifecycleTerm)
				r.emitDecision(EventActorStopped, inst.id, msg.ID, DecisionStop, inst.restarts, string(ActorStopped), ErrLifecycleStartFailed.Error())
				return
			}
			inst.status.Store(int32(ActorRunningCode))
			r.emitDecision(EventActorRestarted, inst.id, msg.ID, decision, inst.restarts, string(ActorRunning), "")
		case DecisionEscalate:
			inst.status.Store(int32(ActorStoppedCode))
			inst.cancel()
			inst.mailbox.Close()
			r.cleanupRegistryForActor(inst.id, RegistryUnregisterLifecycleTerm)
			r.emitDecision(EventActorEscalated, inst.id, msg.ID, decision, inst.restarts, string(ActorStopped), "escalated")
		default:
			inst.status.Store(int32(ActorStoppedCode))
			inst.cancel()
			inst.mailbox.Close()
			r.cleanupRegistryForActor(inst.id, RegistryUnregisterLifecycleTerm)
			r.emitDecision(EventActorStopped, inst.id, msg.ID, decision, inst.restarts, string(ActorStopped), "supervisor_stop")
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
	inst.status.Store(int32(ActorStoppingCode))
	r.runStopHookWithTimeout(inst, inst.cfg.stopHookTimeout)
	inst.status.Store(int32(ActorStoppedCode))
	if v, ok := r.actorPID.Load(actorID); ok {
		r.resolver.SetState(v.(PID), PIDRouteStopped)
	}
	inst.cancel()
	inst.mailbox.Close()
	r.cleanupRegistryForActor(actorID, RegistryUnregisterLifecycleTerm)
	return StopStopped
}

func (r *Runtime) cleanupRegistryForActor(actorID string, result RegistryOperationResult) {
	entries := r.names.unregisterByActor(actorID)
	for _, e := range entries {
		r.metrics.ObserveRegistryOperation(string(result))
		r.emitRegistry(EventRegistryUnregister, e.name, e.actorID, e.pid, result, "")
	}
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
	case ActorStoppingCode:
		return ActorStopping
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

func (r *Runtime) LifecycleOutcomes(actorID string) []LifecycleHookOutcome {
	return r.outcomes.lifecycleByActor(actorID)
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
	return r.sendPIDWithSender(ctx, "", pid, payload)
}

func (r *Runtime) sendPIDWithSender(ctx context.Context, senderActorID string, pid PID, payload any) PIDSendAck {
	start := r.now()
	entry, ok := r.resolver.Resolve(pid)
	r.metrics.ObservePIDLookupLatency(pid.Key(), r.now().Sub(start))
	msgID := r.msgSeq.Add(1)
	mode := r.resolver.GatewayMode()
	guardrailActor := pid.ActorID
	if senderActorID != "" {
		guardrailActor = senderActorID
	}
	if !ok {
		if _, exists := r.registry.get(pid.ActorID); !exists {
			r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedNotFound, string(PIDRejectedNotFound))
			r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, string(PIDRejectedNotFound))
			return PIDSendAck{Outcome: PIDRejectedNotFound, MessageID: msgID}
		}
		r.emitPID(EventPIDUnresolved, pid, msgID, PIDUnresolved, string(PIDUnresolved))
		r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, string(PIDUnresolved))
		return PIDSendAck{Outcome: PIDUnresolved, MessageID: msgID}
	}
	if entry.RouteState == PIDRouteStopped {
		r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedStopped, string(PIDRejectedStopped))
		r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, string(PIDRejectedStopped))
		return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: msgID}
	}
	if pid.Generation != entry.CurrentGeneration {
		r.emitPID(EventPIDRejected, pid, msgID, PIDRejectedStaleGeneration, string(PIDRejectedStaleGeneration))
		r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, string(PIDRejectedStaleGeneration))
		return PIDSendAck{Outcome: PIDRejectedStaleGeneration, MessageID: msgID}
	}
	if mode == GatewayRouteGatewayMediated && !r.resolver.GatewayAvailable() {
		r.emitPID(EventPIDRejected, pid, msgID, PIDUnresolved, ErrGatewayRouteFailed.Error())
		r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, ErrGatewayRouteFailed.Error())
		return PIDSendAck{Outcome: PIDUnresolved, MessageID: msgID}
	}
	ack := r.Send(ctx, pid.ActorID, payload)
	if ack.Result != SubmitAccepted {
		r.emitPID(EventPIDRejected, pid, ack.MessageID, PIDRejectedStopped, string(PIDRejectedStopped))
		r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteFailure, string(PIDRejectedStopped))
		return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: ack.MessageID}
	}
	r.emitPID(EventPIDDelivered, pid, ack.MessageID, PIDDelivered, "")
	r.emitGuardrail(guardrailActor, mode, GuardrailGatewayRouteSuccess, "")
	return PIDSendAck{Outcome: PIDDelivered, MessageID: ack.MessageID}
}

func (r *Runtime) CrossActorSendByActorID(ctx context.Context, senderActorID, targetActorID string, payload any) SubmitAck {
	if r.PIDInteractionPolicy() == PIDInteractionPolicyPIDOnly && senderActorID != "" && senderActorID != targetActorID {
		msgID := r.msgSeq.Add(1)
		r.outcomes.put(ProcessingOutcome{
			MessageID:   msgID,
			ActorID:     targetActorID,
			Result:      ResultRejectedFound,
			CompletedAt: r.now(),
			ErrorCode:   ErrNonPIDCrossActor.Error(),
		})
		r.emitGuardrail(senderActorID, r.resolver.GatewayMode(), GuardrailPolicyRejectNonPID, ErrNonPIDCrossActor.Error())
		return SubmitAck{Result: SubmitRejectedFound, MessageID: msgID}
	}
	r.emitGuardrail(senderActorID, r.resolver.GatewayMode(), GuardrailPolicyAccept, "")
	return r.Send(ctx, targetActorID, payload)
}

func (r *Runtime) CrossActorSendPID(ctx context.Context, senderActorID string, target PID, payload any) PIDSendAck {
	if r.PIDInteractionPolicy() == PIDInteractionPolicyPIDOnly {
		r.emitGuardrail(senderActorID, r.resolver.GatewayMode(), GuardrailPolicyAccept, "")
	}
	return r.sendPIDWithSender(ctx, senderActorID, target, payload)
}

func (r *Runtime) SetPIDInteractionPolicy(mode PIDInteractionPolicyMode) {
	r.policyMu.Lock()
	defer r.policyMu.Unlock()
	r.pidPolicy = mode
}

func (r *Runtime) PIDInteractionPolicy() PIDInteractionPolicyMode {
	r.policyMu.RLock()
	defer r.policyMu.RUnlock()
	return r.pidPolicy
}

func (r *Runtime) SetGatewayRouteMode(mode GatewayRouteMode) {
	r.resolver.SetGatewayMode(mode)
}

func (r *Runtime) SetGatewayAvailable(available bool) {
	r.resolver.SetGatewayAvailability(available)
}

func (r *Runtime) GuardrailOutcomes(actorID string) []GuardrailOutcome {
	return r.outcomes.guardrailByActor(actorID)
}

func (r *Runtime) ValidateDistributedReadiness() []ReadinessValidationRecord {
	mode := r.PIDInteractionPolicy()
	if mode == PIDInteractionPolicyPIDOnly {
		r.emitReadiness(ReadinessScopePIDPolicy, ReadinessPass, "")
	} else {
		r.emitReadiness(ReadinessScopePIDPolicy, ReadinessFail, "pid_policy_not_enforced")
	}
	if r.resolver.GatewayMode() == GatewayRouteLocalDirect || r.resolver.GatewayMode() == GatewayRouteGatewayMediated {
		r.emitReadiness(ReadinessScopeGatewayBoundary, ReadinessPass, "")
	} else {
		r.emitReadiness(ReadinessScopeGatewayBoundary, ReadinessFail, "invalid_gateway_mode")
	}
	r.emitReadiness(ReadinessScopeLocationTransparency, ReadinessPass, "")
	return r.outcomes.readinessAll()
}

func (r *Runtime) RegisterName(actorID, name, namespace string) (RegistryRegisterAck, error) {
	if _, ok := r.registry.get(actorID); !ok {
		return RegistryRegisterAck{Result: RegistryRegisterRejectedDup, Name: name}, ErrActorNotFound
	}
	if namespace == "" {
		namespace = "default"
	}
	pid, ok := r.PIDForActor(actorID)
	if !ok || pid.Namespace != namespace {
		var err error
		pid, err = r.IssuePID(namespace, actorID)
		if err != nil {
			return RegistryRegisterAck{Result: RegistryRegisterRejectedDup, Name: name}, err
		}
	}
	entry, err := r.names.register(name, actorID, pid)
	if err != nil {
		r.metrics.ObserveRegistryOperation(string(RegistryRegisterRejectedDup))
		if err == ErrRegistryDuplicateName {
			r.emitRegistry(EventRegistryRegister, name, actorID, pid, RegistryRegisterRejectedDup, ErrRegistryDuplicateName.Error())
			return RegistryRegisterAck{Result: RegistryRegisterRejectedDup, Name: name}, err
		}
		r.emitRegistry(EventRegistryRegister, name, actorID, pid, RegistryRegisterRejectedDup, err.Error())
		return RegistryRegisterAck{Result: RegistryRegisterRejectedDup, Name: name}, err
	}
	r.metrics.ObserveRegistryOperation(string(RegistryRegisterSuccess))
	r.emitRegistry(EventRegistryRegister, entry.name, entry.actorID, entry.pid, RegistryRegisterSuccess, "")
	return RegistryRegisterAck{Result: RegistryRegisterSuccess, Name: entry.name, PID: entry.pid}, nil
}

func (r *Runtime) LookupName(name string) RegistryLookupAck {
	start := r.now()
	entry, ok := r.names.lookup(name)
	r.metrics.ObserveRegistryLookupLatency(name, r.now().Sub(start))
	if !ok {
		r.metrics.ObserveRegistryOperation(string(RegistryLookupNotFound))
		r.emitRegistry(EventRegistryLookup, name, "", PID{}, RegistryLookupNotFound, ErrRegistryNameNotFound.Error())
		return RegistryLookupAck{Result: RegistryLookupNotFound, Name: name}
	}
	r.metrics.ObserveRegistryOperation(string(RegistryLookupHit))
	r.emitRegistry(EventRegistryLookup, entry.name, entry.actorID, entry.pid, RegistryLookupHit, "")
	return RegistryLookupAck{Result: RegistryLookupHit, Name: entry.name, PID: entry.pid}
}

func (r *Runtime) UnregisterName(name string) RegistryLookupAck {
	entry, ok := r.names.unregister(name)
	if !ok {
		r.metrics.ObserveRegistryOperation(string(RegistryLookupNotFound))
		r.emitRegistry(EventRegistryUnregister, name, "", PID{}, RegistryLookupNotFound, ErrRegistryNameNotFound.Error())
		return RegistryLookupAck{Result: RegistryLookupNotFound, Name: name}
	}
	r.metrics.ObserveRegistryOperation(string(RegistryUnregisterSuccess))
	r.emitRegistry(EventRegistryUnregister, entry.name, entry.actorID, entry.pid, RegistryUnregisterSuccess, "")
	return RegistryLookupAck{Result: RegistryUnregisterSuccess, Name: entry.name, PID: entry.pid}
}
