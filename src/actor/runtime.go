package actor

import (
	"context"
	"fmt"
	"hash/fnv"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tripleclabs/westcoast/src/internal/metrics"
)

type RuntimeOption func(*Runtime)

type ActorOption func(*actorConfig)

type actorConfig struct {
	mailboxCapacity int
	startHook       LifecycleHook
	stopHook        LifecycleHook
	stopHookTimeout time.Duration
	batch           BatchConfig
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
	batch        BatchConfig
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
	askMu     sync.Mutex
	askSeq    atomic.Uint64
	askWait   map[string]pendingAsk
	askReply  map[string]string
	askClosed map[string]string
	routerMu  sync.RWMutex
	routers   map[string]*routerRuntimeState
	randMu    sync.Mutex
	randSrc   *rand.Rand
	brokerMu  sync.RWMutex
	brokers   map[string]*pubsubBrokerService

	// Cluster integration (nil when single-node).
	nodeID     string              // local node identity; "" when no cluster
	remoteSend RemoteSenderFunc    // sends messages to remote nodes
	remoteAsk  RemoteAskSenderFunc // sends ask messages to remote nodes

	// Cluster registry integration (nil when using local-only registry).
	clusterRegister   func(name string, pid PID) error
	clusterLookup     func(name string) (PID, bool)
	clusterUnregister func(name string) (PID, bool)
}

type pendingAsk struct {
	requestID   string
	targetActor string
	replyTo     PID
	waitCh      chan AskReplyEnvelope
}

type routerRuntimeState struct {
	strategy RouterStrategy
	workers  []string
	rrNext   atomic.Uint64
}

const askReplyNamespace = "__ask_reply"

// RemoteSenderFunc sends a message to a remote node. Used by the cluster
// integration layer to inject remote send capability without creating an
// import cycle.
type RemoteSenderFunc func(ctx context.Context, senderActorID string, pid PID, payload any, msgID uint64) (PIDSendAck, error)

// RemoteAskSenderFunc sends an ask request to a remote node.
type RemoteAskSenderFunc func(ctx context.Context, senderActorID string, pid PID, payload any, msgID uint64, askRequestID string, replyTo PID) (PIDSendAck, error)

func WithEmitter(e EventEmitter) RuntimeOption {
	return func(r *Runtime) { r.emitter = e }
}

func WithSupervisor(p SupervisorPolicy) RuntimeOption {
	return func(r *Runtime) { r.policy = p }
}

func WithMetrics(h metrics.Hooks) RuntimeOption {
	return func(r *Runtime) { r.metrics = h }
}

// WithNodeID sets the local node identity. When set, PIDs issued without
// an explicit namespace default to this node ID. Required for cluster operation.
func WithNodeID(id string) RuntimeOption {
	return func(r *Runtime) { r.nodeID = id }
}

// WithRemoteSend injects the function used to send messages to remote nodes.
// This is called by the cluster integration layer during setup.
func WithRemoteSend(fn RemoteSenderFunc) RuntimeOption {
	return func(r *Runtime) { r.remoteSend = fn }
}

// WithRemoteAskSend injects the function used to send ask requests to remote nodes.
func WithRemoteAskSend(fn RemoteAskSenderFunc) RuntimeOption {
	return func(r *Runtime) { r.remoteAsk = fn }
}

// NodeID returns the local node identity, or "" if not clustered.
func (r *Runtime) NodeID() string { return r.nodeID }

// WithClusterRegistry injects cluster-wide registry functions.
// When set, RegisterName/LookupName/UnregisterName delegate to
// these functions in addition to the local registry.
func WithClusterRegistry(
	register func(name string, pid PID) error,
	lookup func(name string) (PID, bool),
	unregister func(name string) (PID, bool),
) RuntimeOption {
	return func(r *Runtime) {
		r.clusterRegister = register
		r.clusterLookup = lookup
		r.clusterUnregister = unregister
	}
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
		askWait:   make(map[string]pendingAsk),
		askReply:  make(map[string]string),
		askClosed: make(map[string]string),
		routers:   make(map[string]*routerRuntimeState),
		randSrc:   rand.New(rand.NewSource(time.Now().UnixNano())),
		brokers:   make(map[string]*pubsubBrokerService),
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

func WithBatching(maxBatchSize int, receiver BatchReceive) ActorOption {
	return func(c *actorConfig) {
		c.batch.Enabled = true
		c.batch.MaxSize = maxBatchSize
		c.batch.Receiver = receiver
	}
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
		batch:        cfg.batch,
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

func (r *Runtime) emitAsk(actorID, requestID string, replyTo PID, outcome AskOutcomeType, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:           eid,
		Type:              EventAskLifecycle,
		ActorID:           actorID,
		RequestID:         requestID,
		ReplyToNamespace:  replyTo.Namespace,
		ReplyToActorID:    replyTo.ActorID,
		ReplyToGeneration: replyTo.Generation,
		Timestamp:         r.now(),
		Result:            string(outcome),
		ErrorCode:         code,
	})
	r.metrics.ObserveAskOutcome(string(outcome))
	r.outcomes.putAsk(AskOutcome{
		RequestID:   requestID,
		ActorID:     actorID,
		ReplyTo:     replyTo,
		Outcome:     outcome,
		ReasonCode:  code,
		CompletedAt: r.now(),
	})
}

func (r *Runtime) emitRouter(routerID string, messageID uint64, strategy RouterStrategy, selectedWorker string, outcome RoutingOutcomeType, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:        eid,
		Type:           EventRouterLifecycle,
		ActorID:        routerID,
		MessageID:      messageID,
		RouterStrategy: string(strategy),
		SelectedWorker: selectedWorker,
		Timestamp:      r.now(),
		Result:         string(outcome),
		ErrorCode:      code,
	})
	r.metrics.ObserveRouterOutcome(string(strategy), string(outcome))
	r.outcomes.putRouting(RoutingOutcome{
		RouterID:       routerID,
		MessageID:      messageID,
		Strategy:       strategy,
		SelectedWorker: selectedWorker,
		Outcome:        outcome,
		ReasonCode:     code,
		At:             r.now(),
	})
}

func (r *Runtime) emitBatch(actorID string, batchSize int, result BatchResult, code string) {
	eid := r.eventSeq.Add(1)
	r.emitter.Emit(Event{
		EventID:   eid,
		Type:      EventBatchLifecycle,
		ActorID:   actorID,
		BatchSize: batchSize,
		Timestamp: r.now(),
		Result:    string(result),
		ErrorCode: code,
	})
	r.metrics.ObserveBatchOutcome(string(result))
	r.outcomes.putBatch(BatchOutcome{
		ActorID:     actorID,
		BatchSize:   batchSize,
		Result:      result,
		ReasonCode:  code,
		CompletedAt: r.now(),
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
				inst.mu.RLock()
				batchCfg := inst.batch
				inst.mu.RUnlock()
				if batchCfg.Enabled && batchCfg.MaxSize > 1 {
					msgs := inst.mailbox.DequeueBatch(batchCfg.MaxSize)
					if len(msgs) == 0 {
						break
					}
					r.processBatch(inst, msgs)
					continue
				}
				msg, ok := inst.mailbox.Dequeue()
				if !ok {
					break
				}
				r.processMessage(inst, msg)
			}
		}
	}
}

func (r *Runtime) processBatch(inst *actorInstance, msgs []Message) {
	if len(msgs) == 0 {
		return
	}
	started := r.now()
	defer func() {
		r.metrics.ObserveProcessingLatency(inst.id, r.now().Sub(started))
	}()

	inst.mu.RLock()
	cfg := inst.batch
	cur := inst.state.value()
	inst.mu.RUnlock()
	if !cfg.Enabled || cfg.Receiver == nil {
		for _, msg := range msgs {
			r.processMessage(inst, msg)
		}
		return
	}

	payloads := make([]any, len(msgs))
	for i, msg := range msgs {
		payloads[i] = msg.Payload
	}

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
		next, err = cfg.Receiver.BatchReceive(inst.ctx, cur, payloads)
	}()
	if err != nil {
		r.metrics.ObservePanicIntercept(inst.id)
		r.emitBatch(inst.id, len(msgs), BatchResultFailedHandler, "batch_handler_error")
		decision := r.policy.Decide(inst.id, err, inst.restarts)
		for _, msg := range msgs {
			r.outcomes.put(ProcessingOutcome{
				MessageID:            msg.ID,
				ActorID:              inst.id,
				Result:               ResultFailed,
				SupervisionDecision:  decision,
				SupervisionIteration: inst.restarts,
				CompletedAt:          r.now(),
				ErrorCode:            "batch_handler_error",
			})
		}
		r.emitDecision(EventActorFailed, inst.id, msgs[0].ID, decision, inst.restarts, string(ResultFailed), "batch_handler_error")
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
				r.emitBatch(inst.id, len(msgs), BatchResultFailedSupervision, ErrLifecycleStartFailed.Error())
				r.emitDecision(EventActorStopped, inst.id, msgs[0].ID, DecisionStop, inst.restarts, string(ActorStopped), ErrLifecycleStartFailed.Error())
				return
			}
			inst.status.Store(int32(ActorRunningCode))
			r.emitDecision(EventActorRestarted, inst.id, msgs[0].ID, decision, inst.restarts, string(ActorRunning), "")
		case DecisionEscalate:
			inst.status.Store(int32(ActorStoppedCode))
			inst.cancel()
			inst.mailbox.Close()
			r.cleanupRegistryForActor(inst.id, RegistryUnregisterLifecycleTerm)
			r.emitBatch(inst.id, len(msgs), BatchResultFailedSupervision, "escalated")
			r.emitDecision(EventActorEscalated, inst.id, msgs[0].ID, decision, inst.restarts, string(ActorStopped), "escalated")
		default:
			inst.status.Store(int32(ActorStoppedCode))
			inst.cancel()
			inst.mailbox.Close()
			r.cleanupRegistryForActor(inst.id, RegistryUnregisterLifecycleTerm)
			r.emitBatch(inst.id, len(msgs), BatchResultFailedSupervision, "supervisor_stop")
			r.emitDecision(EventActorStopped, inst.id, msgs[0].ID, decision, inst.restarts, string(ActorStopped), "supervisor_stop")
		}
		return
	}

	inst.mu.Lock()
	inst.state.apply(next)
	inst.mu.Unlock()
	for _, msg := range msgs {
		r.outcomes.put(ProcessingOutcome{MessageID: msg.ID, ActorID: inst.id, Result: ResultDelivered, CompletedAt: r.now()})
		r.emitLocal(EventMessageProcessed, inst.id, msg, string(ResultDelivered), "")
	}
	r.emitBatch(inst.id, len(msgs), BatchResultSuccess, "")
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

func (r *Runtime) ConfigureBatching(actorID string, maxBatchSize int, receiver BatchReceive) error {
	inst, ok := r.registry.get(actorID)
	if !ok {
		return ErrActorNotFound
	}
	if maxBatchSize <= 0 {
		return ErrBatchConfigInvalid
	}
	inst.mu.Lock()
	inst.batch.Enabled = true
	inst.batch.MaxSize = maxBatchSize
	inst.batch.Receiver = receiver
	inst.cfg.batch = inst.batch
	inst.mu.Unlock()
	return nil
}

func (r *Runtime) DisableBatching(actorID string) error {
	inst, ok := r.registry.get(actorID)
	if !ok {
		return ErrActorNotFound
	}
	inst.mu.Lock()
	inst.batch = BatchConfig{}
	inst.cfg.batch = inst.batch
	inst.mu.Unlock()
	return nil
}

func (r *Runtime) BatchOutcomes(actorID string) []BatchOutcome {
	return r.outcomes.batchByActor(actorID)
}

func (r *Runtime) ConfigureRouter(routerID string, strategy RouterStrategy, workers []string) error {
	if _, ok := r.registry.get(routerID); !ok {
		return ErrActorNotFound
	}
	switch strategy {
	case RouterStrategyRoundRobin, RouterStrategyRandom, RouterStrategyConsistentKey:
	default:
		return fmt.Errorf("invalid router strategy: %s", strategy)
	}
	normalized := make([]string, 0, len(workers))
	seen := make(map[string]struct{}, len(workers))
	for _, worker := range workers {
		if worker == "" {
			continue
		}
		if _, ok := seen[worker]; ok {
			continue
		}
		seen[worker] = struct{}{}
		normalized = append(normalized, worker)
	}
	r.routerMu.Lock()
	state, ok := r.routers[routerID]
	if !ok {
		state = &routerRuntimeState{}
		r.routers[routerID] = state
	}
	state.strategy = strategy
	state.workers = append(state.workers[:0], normalized...)
	state.rrNext.Store(0)
	r.routerMu.Unlock()
	return nil
}

func (r *Runtime) RoutingOutcomes(routerID string) []RoutingOutcome {
	return r.outcomes.routingByRouter(routerID)
}

func (r *Runtime) Route(ctx context.Context, routerID string, payload any) SubmitAck {
	return r.sendWithAskContext(ctx, routerID, payload, nil)
}

func (r *Runtime) routerState(actorID string) (RouterStrategy, []string, *atomic.Uint64, bool) {
	r.routerMu.RLock()
	defer r.routerMu.RUnlock()
	state, ok := r.routers[actorID]
	if !ok {
		return "", nil, nil, false
	}
	return state.strategy, append([]string(nil), state.workers...), &state.rrNext, true
}

func (r *Runtime) selectRoutedWorker(strategy RouterStrategy, workers []string, rrCounter *atomic.Uint64, payload any) (string, RoutingOutcomeType, string) {
	if len(workers) == 0 {
		return "", RouteFailedNoWorkers, "router_no_workers"
	}
	switch strategy {
	case RouterStrategyRoundRobin:
		idx := rrCounter.Add(1) - 1
		worker := workers[int(idx%uint64(len(workers)))]
		return worker, RouteSuccess, ""
	case RouterStrategyRandom:
		r.randMu.Lock()
		idx := r.randSrc.Intn(len(workers))
		r.randMu.Unlock()
		return workers[idx], RouteSuccess, ""
	case RouterStrategyConsistentKey:
		msg, ok := payload.(HashKeyMessage)
		if !ok {
			return "", RouteFailedInvalidKey, "router_missing_hash_key"
		}
		key := msg.HashKey()
		if key == "" {
			return "", RouteFailedInvalidKey, "router_empty_hash_key"
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(key))
		idx := int(h.Sum32() % uint32(len(workers)))
		return workers[idx], RouteSuccess, ""
	default:
		return "", RouteFailedInvalidKey, "router_invalid_strategy"
	}
}

func (r *Runtime) dispatchRouted(ctx context.Context, routerID string, payload any, askCtx *AskRequestContext, strategy RouterStrategy, workers []string, rrCounter *atomic.Uint64) SubmitAck {
	msgID := r.msgSeq.Add(1)
	selectedWorker, outcome, reason := r.selectRoutedWorker(strategy, workers, rrCounter, payload)
	if outcome != RouteSuccess {
		r.emitRouter(routerID, msgID, strategy, "", outcome, reason)
		return SubmitAck{Result: SubmitRejectedFound, MessageID: msgID}
	}
	ack := r.sendWithAskContext(ctx, selectedWorker, payload, askCtx)
	if ack.Result != SubmitAccepted {
		r.emitRouter(routerID, ack.MessageID, strategy, selectedWorker, RouteFailedWorkerUnavailable, string(ack.Result))
		return ack
	}
	r.emitRouter(routerID, ack.MessageID, strategy, selectedWorker, RouteSuccess, "")
	return ack
}

func (r *Runtime) Send(ctx context.Context, actorID string, payload any) SubmitAck {
	return r.sendWithAskContext(ctx, actorID, payload, nil)
}

func (r *Runtime) sendWithAskContext(ctx context.Context, actorID string, payload any, askCtx *AskRequestContext) SubmitAck {
	sendStart := r.now()
	defer func() {
		r.metrics.ObserveLocalSendLatency(actorID, r.now().Sub(sendStart))
	}()

	if payload == nil {
		msgID := r.msgSeq.Add(1)
		r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedNilPayload, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedNilPayload)})
		r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedNilPayload), string(SubmitRejectedNilPayload))
		r.metrics.ObserveLocalRouting(actorID, string(SubmitRejectedNilPayload))
		return SubmitAck{Result: SubmitRejectedNilPayload, MessageID: msgID}
	}

	if strategy, workers, rrCounter, ok := r.routerState(actorID); ok {
		inst, exists := r.registry.get(actorID)
		msgID := r.msgSeq.Add(1)
		if !exists {
			r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedFound, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedFound)})
			r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedFound), string(SubmitRejectedFound))
			return SubmitAck{Result: SubmitRejectedFound, MessageID: msgID}
		}
		if actorStatusCode(inst.status.Load()) == ActorStoppedCode {
			r.outcomes.put(ProcessingOutcome{MessageID: msgID, ActorID: actorID, Result: ResultRejectedStop, CompletedAt: r.now(), ErrorCode: string(SubmitRejectedStop)})
			r.emit(EventMessageRejected, actorID, msgID, string(ResultRejectedStop), string(SubmitRejectedStop))
			return SubmitAck{Result: SubmitRejectedStop, MessageID: msgID}
		}
		return r.dispatchRouted(ctx, actorID, payload, askCtx, strategy, workers, rrCounter)
	}

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

	typeName := messageTypeName(payload)
	schemaVersion := messageSchemaVersion(payload)
	msg := Message{
		ID:            msgID,
		ActorID:       actorID,
		Payload:       payload,
		Ask:           askCtx,
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

func (r *Runtime) newAskRequest(targetActor string) pendingAsk {
	requestID := fmt.Sprintf("ask-%d", r.askSeq.Add(1))
	// When clustered, qualify the ask-reply namespace with the node ID
	// so remote nodes know where to send the reply back.
	ns := askReplyNamespace
	if r.nodeID != "" {
		ns = askReplyNamespace + "@" + r.nodeID
	}
	replyTo := PID{Namespace: ns, ActorID: requestID, Generation: 1}
	wait := pendingAsk{
		requestID:   requestID,
		targetActor: targetActor,
		replyTo:     replyTo,
		waitCh:      make(chan AskReplyEnvelope, 1),
	}
	r.askMu.Lock()
	r.askWait[requestID] = wait
	r.askReply[replyTo.Key()] = requestID
	r.askMu.Unlock()
	return wait
}

func (r *Runtime) completeAskRequest(wait pendingAsk) {
	r.askMu.Lock()
	delete(r.askWait, wait.requestID)
	delete(r.askReply, wait.replyTo.Key())
	r.askClosed[wait.requestID] = wait.targetActor
	if len(r.askClosed) > 4096 {
		for k := range r.askClosed {
			delete(r.askClosed, k)
			break
		}
	}
	r.askMu.Unlock()
}

func (r *Runtime) Ask(ctx context.Context, actorID string, payload any, timeout time.Duration) (AskResult, error) {
	if timeout <= 0 {
		return AskResult{}, ErrAskInvalidTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}
	wait := r.newAskRequest(actorID)
	ack := r.sendWithAskContext(ctx, actorID, payload, &AskRequestContext{
		RequestID: wait.requestID,
		ReplyTo:   wait.replyTo,
	})
	if ack.Result != SubmitAccepted {
		r.completeAskRequest(wait)
		r.emitAsk(actorID, wait.requestID, wait.replyTo, AskOutcomeReplyTargetInvalid, string(ack.Result))
		return AskResult{}, ErrAskReplyTargetInvalid
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case rep := <-wait.waitCh:
		r.completeAskRequest(wait)
		r.emitAsk(actorID, wait.requestID, wait.replyTo, AskOutcomeSuccess, "")
		return AskResult{RequestID: rep.RequestID, Payload: rep.Payload}, nil
	case <-timer.C:
		r.completeAskRequest(wait)
		r.emitAsk(actorID, wait.requestID, wait.replyTo, AskOutcomeTimeout, ErrAskTimeout.Error())
		return AskResult{}, ErrAskTimeout
	case <-ctx.Done():
		r.completeAskRequest(wait)
		r.emitAsk(actorID, wait.requestID, wait.replyTo, AskOutcomeCanceled, ErrAskCanceled.Error())
		return AskResult{}, ErrAskCanceled
	}
}

func (r *Runtime) routeAskReply(pid PID, payload any, msgID uint64) (PIDSendAck, bool) {
	// Match both "__ask_reply" and "__ask_reply@<nodeID>".
	if !isAskReplyNamespace(pid.Namespace) {
		return PIDSendAck{}, false
	}
	// If the ask-reply is for a different node, don't handle locally —
	// let sendPIDWithSender route it remotely.
	if r.nodeID != "" && pid.Namespace != askReplyNamespace && pid.Namespace != askReplyNamespace+"@"+r.nodeID {
		return PIDSendAck{}, false
	}
	replyKey := pid.Key()
	r.askMu.Lock()
	requestID, ok := r.askReply[replyKey]
	if !ok {
		targetActor, closed := r.askClosed[pid.ActorID]
		r.askMu.Unlock()
		if closed {
			r.emitAsk(targetActor, pid.ActorID, pid, AskOutcomeLateReplyDropped, "ask_wait_closed")
			return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: msgID}, true
		}
		r.emitAsk("", pid.ActorID, pid, AskOutcomeReplyTargetInvalid, ErrAskReplyTargetInvalid.Error())
		return PIDSendAck{Outcome: PIDRejectedNotFound, MessageID: msgID}, true
	}
	wait := r.askWait[requestID]
	delete(r.askWait, requestID)
	delete(r.askReply, replyKey)
	r.askClosed[requestID] = wait.targetActor
	r.askMu.Unlock()

	env, isEnvelope := payload.(AskReplyEnvelope)
	if !isEnvelope {
		env = AskReplyEnvelope{RequestID: requestID, Payload: payload, RepliedAt: r.now()}
	}
	if env.RequestID == "" {
		env.RequestID = requestID
	}
	if env.RequestID != requestID {
		r.emitAsk(wait.targetActor, requestID, pid, AskOutcomeReplyTargetInvalid, "ask_request_mismatch")
		return PIDSendAck{Outcome: PIDRejectedNotFound, MessageID: msgID}, true
	}
	select {
	case wait.waitCh <- env:
		return PIDSendAck{Outcome: PIDDelivered, MessageID: msgID}, true
	default:
		r.emitAsk(wait.targetActor, requestID, pid, AskOutcomeLateReplyDropped, "ask_duplicate_reply")
		return PIDSendAck{Outcome: PIDRejectedStopped, MessageID: msgID}, true
	}
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
	// Default to the local node ID when cluster is active.
	if namespace == "" && r.nodeID != "" {
		namespace = r.nodeID
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
	msgID := r.msgSeq.Add(1)
	if askAck, handled := r.routeAskReply(pid, payload, msgID); handled {
		if askAck.Outcome == PIDDelivered {
			r.emitPID(EventPIDDelivered, pid, msgID, askAck.Outcome, "")
		} else {
			r.emitPID(EventPIDRejected, pid, msgID, askAck.Outcome, string(askAck.Outcome))
		}
		return askAck
	}

	// Remote routing: if the PID targets a different node, send via transport.
	if r.remoteSend != nil && pid.IsRemote(r.nodeID) {
		ack, err := r.remoteSend(ctx, senderActorID, pid, payload, msgID)
		if err != nil {
			r.emitPID(EventPIDRejected, pid, msgID, PIDUnresolved, err.Error())
			return ack
		}
		r.emitPID(EventPIDDelivered, pid, msgID, PIDDelivered, "")
		return ack
	}

	start := r.now()
	entry, ok := r.resolver.Resolve(pid)
	r.metrics.ObservePIDLookupLatency(pid.Key(), r.now().Sub(start))
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

func (r *Runtime) AskOutcomes(actorID string) []AskOutcome {
	return r.outcomes.askByActor(actorID)
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

	// Propagate to cluster registry if available.
	if r.clusterRegister != nil {
		r.clusterRegister(name, entry.pid)
	}

	return RegistryRegisterAck{Result: RegistryRegisterSuccess, Name: entry.name, PID: entry.pid}, nil
}

func (r *Runtime) LookupName(name string) RegistryLookupAck {
	start := r.now()
	entry, ok := r.names.lookup(name)
	r.metrics.ObserveRegistryLookupLatency(name, r.now().Sub(start))
	if !ok {
		// Fall back to cluster registry if available.
		if r.clusterLookup != nil {
			if pid, found := r.clusterLookup(name); found {
				r.metrics.ObserveRegistryOperation(string(RegistryLookupHit))
				r.emitRegistry(EventRegistryLookup, name, pid.ActorID, pid, RegistryLookupHit, "cluster")
				return RegistryLookupAck{Result: RegistryLookupHit, Name: name, PID: pid}
			}
		}
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

	// Propagate to cluster registry.
	if r.clusterUnregister != nil {
		r.clusterUnregister(name)
	}

	return RegistryLookupAck{Result: RegistryUnregisterSuccess, Name: entry.name, PID: entry.pid}
}

// DeliverLocal delivers an inbound message from a remote node to a local actor.
// This implements the RuntimeBridge interface used by the cluster InboundDispatcher.
func (r *Runtime) DeliverLocal(ctx context.Context, actorID string, payload any, askCtx *AskRequestContext) SubmitAck {
	return r.sendWithAskContext(ctx, actorID, payload, askCtx)
}

// DeliverPID delivers a message to a local PID. Used for ask-reply routing
// when a reply arrives from a remote node.
func (r *Runtime) DeliverPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	return r.SendPID(ctx, pid, payload)
}
