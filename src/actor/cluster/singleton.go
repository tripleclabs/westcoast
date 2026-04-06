package cluster

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// System envelope type names for singleton handoff protocol.
const (
	singletonHandoffReqType  = "__singleton_handoff_req"
	singletonHandoffRespType = "__singleton_handoff_resp"
)

// singletonPhase tracks the lifecycle of a singleton on this node.
type singletonPhase int

const (
	phaseIdle         singletonPhase = iota // not running on this node
	phaseActivating                         // new leader, waiting for handoff response
	phaseActive                             // running on this node
	phaseDeactivating                       // lost leadership, awaiting handoff request or safety timeout
)

// SingletonSpec defines an actor that should run on exactly one node
// in the cluster at any time.
type SingletonSpec struct {
	// Name is both the actor ID and the registered name.
	Name         string
	InitialState any
	Handler      actor.Handler
	Options      []actor.ActorOption
	// Placement restricts which nodes can host this singleton. If nil,
	// any node is eligible. When set, only matching nodes participate
	// in the leader election for this singleton's scope.
	Placement NodeMatcher

	// HandoffTimeout is how long the new leader waits for the old leader
	// to stop the singleton and reply with handoff state. If the old leader
	// does not respond within this duration, the new leader starts the
	// singleton anyway. Defaults to 10s.
	HandoffTimeout time.Duration

	// OnHandoff is called on the NEW leader when it receives state from
	// the old leader. The returned value becomes InitialState for the new
	// instance. If nil, handoff state is ignored and InitialState is used.
	OnHandoff func(handoffState any) any

	// CaptureState is called on the OLD leader during deactivation, after
	// the singleton actor has been stopped. It returns the state to transfer
	// to the new leader. If nil, no state is transferred.
	CaptureState func(ctx context.Context, actorID string) (any, error)
}

func (s *SingletonSpec) handoffTimeout() time.Duration {
	if s.HandoffTimeout > 0 {
		return s.HandoffTimeout
	}
	return 10 * time.Second
}

// singletonState tracks a singleton's lifecycle on this node.
type singletonState struct {
	spec   SingletonSpec
	phase  singletonPhase
	term   uint64             // election term when current phase was entered
	cancel context.CancelFunc // cancel in-progress lifecycle operation
}

// singletonHandoffRequest is the wire message sent from the new leader
// to the old leader requesting that it stop the singleton and return
// its state.
type singletonHandoffRequest struct {
	Name      string
	Term      uint64
	NewLeader NodeID
}

// singletonHandoffResponse is the wire reply from the old leader
// confirming that the singleton has been stopped.
type singletonHandoffResponse struct {
	Name  string
	Term  uint64
	State any    // nil if CaptureState not configured or failed
	OK    bool   // false if singleton wasn't running or term mismatch
	Error string // reason if !OK
}

// SingletonManager ensures that a set of singleton actors run on
// exactly one node — the current leader for each singleton's scope.
// When leadership changes, the handoff protocol coordinates stopping
// on the old leader before starting on the new one.
type SingletonManager struct {
	runtime    *actor.Runtime
	election   *RingElection
	registry   *DistributedRegistry
	cluster    *Cluster
	codec      Codec
	dispatcher *InboundDispatcher

	mu         sync.Mutex
	singletons map[string]*singletonState
	cancel     context.CancelFunc

	// Recently failed nodes — skip handoff for these.
	failedMu    sync.Mutex
	failedNodes map[NodeID]time.Time

	// Pending handoff request correlation.
	handoffMu    sync.Mutex
	handoffWaits map[string]chan singletonHandoffResponse

	// Previous leaders per scope, populated by watchLoop.
	prevLeaders map[string]NodeID
}

// SingletonManagerOption configures optional SingletonManager behavior.
type SingletonManagerOption func(*SingletonManager)

// WithCluster enables placement-aware leader election and cross-node
// handoff coordination.
func WithCluster(c *Cluster) SingletonManagerOption {
	return func(sm *SingletonManager) { sm.cluster = c }
}

// WithCodec sets the codec for encoding handoff messages.
func WithCodec(c Codec) SingletonManagerOption {
	return func(sm *SingletonManager) { sm.codec = c }
}

// WithDispatcher registers the singleton handoff protocol handlers
// with the inbound dispatcher for cross-node communication.
func WithDispatcher(d *InboundDispatcher) SingletonManagerOption {
	return func(sm *SingletonManager) { sm.dispatcher = d }
}

// NewSingletonManager creates a SingletonManager. Provide WithCluster,
// WithCodec, and WithDispatcher options to enable the handoff protocol.
// Without these, the manager falls back to immediate start/stop (suitable
// for single-node or when at-most-one is not critical).
func NewSingletonManager(runtime *actor.Runtime, election *RingElection, registry *DistributedRegistry, opts ...SingletonManagerOption) *SingletonManager {
	sm := &SingletonManager{
		runtime:      runtime,
		election:     election,
		registry:     registry,
		singletons:   make(map[string]*singletonState),
		failedNodes:  make(map[NodeID]time.Time),
		handoffWaits: make(map[string]chan singletonHandoffResponse),
	}
	for _, opt := range opts {
		opt(sm)
	}
	return sm
}

// handoffEnabled returns true if the manager has the dependencies needed
// for the cross-node handoff protocol.
func (sm *SingletonManager) handoffEnabled() bool {
	return sm.cluster != nil && sm.codec != nil && sm.dispatcher != nil
}

// Register adds a singleton spec. The actor will be started if this node
// is currently the leader for the singleton's scope.
func (sm *SingletonManager) Register(spec SingletonSpec) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.singletons[spec.Name] = &singletonState{spec: spec}
}

// Start begins watching leadership changes and starts singletons
// that this node is responsible for.
func (sm *SingletonManager) Start(ctx context.Context) {
	ctx, sm.cancel = context.WithCancel(ctx)

	// Register system envelope handlers for handoff protocol.
	if sm.handoffEnabled() {
		sm.registerHandoffTypes()
		sm.dispatcher.RegisterHandler(singletonHandoffReqType, sm.onHandoffRequest)
		sm.dispatcher.RegisterHandler(singletonHandoffRespType, sm.onHandoffResponse)
	}

	sm.mu.Lock()
	specs := make([]string, 0, len(sm.singletons))
	for name := range sm.singletons {
		specs = append(specs, name)
	}
	sm.mu.Unlock()

	// Watch each singleton's scope.
	for _, name := range specs {
		ch := sm.election.Watch("singleton/" + name)
		go sm.watchLoop(ctx, name, ch)
	}

	// Reconcile immediately in case we're already the leader.
	sm.reconcile()
}

// registerHandoffTypes registers the handoff wire types with the codec
// so they can be encoded/decoded through gob.
func (sm *SingletonManager) registerHandoffTypes() {
	sm.codec.Register(singletonHandoffRequest{})
	sm.codec.Register(singletonHandoffResponse{})
}

// Stop terminates all managed singletons and stops watching.
func (sm *SingletonManager) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for name, s := range sm.singletons {
		// Cancel any in-progress lifecycle operation.
		if s.cancel != nil {
			s.cancel()
			s.cancel = nil
		}
		if s.phase == phaseActive || s.phase == phaseDeactivating {
			sm.runtime.Stop(name)
			s.phase = phaseIdle
		}
	}
}

// Running returns the names of singletons currently running on this node.
func (sm *SingletonManager) Running() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var out []string
	for name, s := range sm.singletons {
		if s.phase == phaseActive || s.phase == phaseDeactivating {
			out = append(out, name)
		}
	}
	return out
}

// OnMemberEvent should be called when a cluster membership event occurs.
// It tracks failed nodes so the handoff protocol can skip unreachable
// nodes and start singletons immediately.
func (sm *SingletonManager) OnMemberEvent(event MemberEvent) {
	if event.Type == MemberFailed {
		sm.failedMu.Lock()
		sm.failedNodes[event.Member.ID] = time.Now()
		sm.failedMu.Unlock()
	}
	// Clean up stale entries older than 5 minutes.
	sm.failedMu.Lock()
	for id, t := range sm.failedNodes {
		if time.Since(t) > 5*time.Minute {
			delete(sm.failedNodes, id)
		}
	}
	sm.failedMu.Unlock()
}

func (sm *SingletonManager) isRecentlyFailed(id NodeID) bool {
	sm.failedMu.Lock()
	defer sm.failedMu.Unlock()
	_, ok := sm.failedNodes[id]
	return ok
}

func (sm *SingletonManager) watchLoop(ctx context.Context, name string, ch <-chan LeaderEvent) {
	scope := "singleton/" + name
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if ev.PrevLeader != "" {
				sm.setPrevLeader(scope, ev.PrevLeader)
			}
			sm.reconcile()
		}
	}
}

// reconcile checks each singleton and transitions its phase based on
// current leadership state.
func (sm *SingletonManager) reconcile() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	nodeID := sm.runtime.NodeID()

	for name, s := range sm.singletons {
		scope := "singleton/" + name
		var isLeader bool

		if s.spec.Placement != nil && sm.cluster != nil {
			members := sm.cluster.Members()
			members = append(members, sm.cluster.Self())
			leader, ok := sm.election.LeaderAmong(scope, s.spec.Placement, members)
			isLeader = ok && string(leader) == nodeID
		} else {
			isLeader = sm.election.IsLeader(scope)
		}

		currentTerm := sm.election.Term(scope)

		if isLeader {
			switch s.phase {
			case phaseIdle:
				sm.gainLeadership(name, s, currentTerm)
			case phaseActivating:
				// Already activating — check if term changed.
				if s.term != currentTerm {
					// Term changed while activating. Cancel and restart.
					if s.cancel != nil {
						s.cancel()
						s.cancel = nil
					}
					sm.gainLeadership(name, s, currentTerm)
				}
			case phaseActive:
				// Still leader, nothing to do.
			case phaseDeactivating:
				// We lost leadership but then regained it before handoff completed.
				if s.cancel != nil {
					s.cancel()
					s.cancel = nil
				}
				s.phase = phaseActive
				s.term = currentTerm
			}
		} else {
			switch s.phase {
			case phaseActive:
				sm.loseLeadership(name, s, currentTerm)
			case phaseActivating:
				// We were trying to activate but lost leadership.
				if s.cancel != nil {
					s.cancel()
					s.cancel = nil
				}
				s.phase = phaseIdle
			case phaseDeactivating:
				// Already deactivating — check if term changed.
				if s.term != currentTerm {
					if s.cancel != nil {
						s.cancel()
						s.cancel = nil
					}
					sm.loseLeadership(name, s, currentTerm)
				}
			case phaseIdle:
				// Not our problem.
			}
		}
	}
}

// gainLeadership handles the transition to leadership for a singleton.
// Must be called with sm.mu held.
func (sm *SingletonManager) gainLeadership(name string, s *singletonState, term uint64) {
	scope := "singleton/" + name

	// Determine the previous leader.
	// We look at the election event — PrevLeader tells us who had it before.
	prevLeader := sm.findPrevLeader(scope)

	// Cases where we can start immediately (no handoff needed):
	// 1. No previous leader (first election or cluster-of-one)
	// 2. Previous leader is us (shouldn't happen, but safe)
	// 3. Previous leader recently failed (node crashed)
	// 4. Handoff not enabled (no codec/dispatcher/cluster)
	canSkipHandoff := prevLeader == "" ||
		prevLeader == NodeID(sm.runtime.NodeID()) ||
		sm.isRecentlyFailed(prevLeader) ||
		!sm.handoffEnabled()

	if canSkipHandoff {
		sm.startActor(name, s, s.spec.InitialState)
		s.phase = phaseActive
		s.term = term
		return
	}

	// Handoff required — send request to old leader.
	s.phase = phaseActivating
	s.term = term

	ctx, cancel := context.WithTimeout(context.Background(), s.spec.handoffTimeout())
	s.cancel = cancel

	go sm.runHandoff(ctx, name, prevLeader, term)
}

// loseLeadership handles the transition away from leadership.
// Must be called with sm.mu held.
func (sm *SingletonManager) loseLeadership(name string, s *singletonState, term uint64) {
	if !sm.handoffEnabled() {
		// No handoff protocol — stop immediately (legacy behavior).
		sm.stopAndCleanup(name, s)
		return
	}

	// Wait for a handoff request from the new leader, or timeout.
	s.phase = phaseDeactivating
	s.term = term

	ctx, cancel := context.WithTimeout(context.Background(), s.spec.handoffTimeout()+5*time.Second)
	s.cancel = cancel

	go sm.runDeactivationTimeout(ctx, name)
}

// runHandoff is the new leader's goroutine that sends a handoff request
// to the old leader and waits for the response.
func (sm *SingletonManager) runHandoff(ctx context.Context, name string, prevLeader NodeID, term uint64) {
	correlationID := fmt.Sprintf("handoff:%s:%d", name, term)

	// Create response channel.
	respCh := make(chan singletonHandoffResponse, 1)
	sm.handoffMu.Lock()
	sm.handoffWaits[correlationID] = respCh
	sm.handoffMu.Unlock()

	defer func() {
		sm.handoffMu.Lock()
		delete(sm.handoffWaits, correlationID)
		sm.handoffMu.Unlock()
	}()

	// Send handoff request.
	req := singletonHandoffRequest{
		Name:      name,
		Term:      term,
		NewLeader: NodeID(sm.runtime.NodeID()),
	}
	if err := sm.sendSystemEnvelope(ctx, prevLeader, singletonHandoffReqType, req, correlationID); err != nil {
		// Can't reach old leader — start anyway after a brief pause.
		select {
		case <-ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	} else {
		// Wait for response or timeout.
		select {
		case <-ctx.Done():
			sm.completeActivation(name, term, nil)
			return
		case resp := <-respCh:
			if resp.OK && resp.State != nil {
				sm.completeActivation(name, term, resp.State)
				return
			}
		}
	}

	sm.completeActivation(name, term, nil)
}

// completeActivation finishes the handoff by starting the singleton on
// this node if the term hasn't changed.
func (sm *SingletonManager) completeActivation(name string, expectedTerm uint64, handoffState any) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.singletons[name]
	if !ok {
		return
	}
	if s.phase != phaseActivating || s.term != expectedTerm {
		// Leadership changed while we were waiting — abort.
		return
	}

	// Verify we're still the leader.
	scope := "singleton/" + name
	currentTerm := sm.election.Term(scope)
	if currentTerm != expectedTerm {
		s.phase = phaseIdle
		return
	}

	initialState := s.spec.InitialState
	if handoffState != nil && s.spec.OnHandoff != nil {
		initialState = s.spec.OnHandoff(handoffState)
	}

	sm.startActor(name, s, initialState)
	s.phase = phaseActive
}

// runDeactivationTimeout waits for the deactivation timeout to expire.
// If no handoff request arrives, the singleton is stopped unilaterally.
func (sm *SingletonManager) runDeactivationTimeout(ctx context.Context, name string) {
	<-ctx.Done()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.singletons[name]
	if !ok {
		return
	}
	if s.phase != phaseDeactivating {
		return
	}

	// Timeout expired and no handoff request arrived — stop unilaterally.
	sm.stopAndCleanup(name, s)
}

// onHandoffRequest handles an incoming handoff request from the new leader.
func (sm *SingletonManager) onHandoffRequest(from NodeID, env Envelope) {
	var decoded any
	if err := sm.codec.Decode(env.Payload, &decoded); err != nil {
		return
	}
	req, ok := decoded.(singletonHandoffRequest)
	if !ok {
		return
	}

	sm.mu.Lock()
	s, exists := sm.singletons[req.Name]
	if !exists {
		sm.mu.Unlock()
		sm.sendHandoffResponse(from, req.Name, req.Term, env.AskRequestID, false, nil, "singleton_not_registered")
		return
	}

	if s.phase != phaseActive && s.phase != phaseDeactivating {
		sm.mu.Unlock()
		sm.sendHandoffResponse(from, req.Name, req.Term, env.AskRequestID, false, nil, "singleton_not_running")
		return
	}

	// Stop the actor while holding the lock to prevent races.
	sm.runtime.Stop(req.Name)

	// Capture state if configured.
	var handoffState any
	if s.spec.CaptureState != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		st, err := s.spec.CaptureState(ctx, req.Name)
		cancel()
		if err == nil {
			handoffState = st
		}
	}

	// Cancel deactivation timeout if running.
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	s.phase = phaseIdle

	// Unregister from distributed registry.
	if sm.registry != nil {
		sm.registry.Unregister(req.Name)
	}

	sm.mu.Unlock()

	sm.sendHandoffResponse(from, req.Name, req.Term, env.AskRequestID, true, handoffState, "")
}

// onHandoffResponse handles an incoming handoff response from the old leader.
func (sm *SingletonManager) onHandoffResponse(from NodeID, env Envelope) {
	var decoded any
	if err := sm.codec.Decode(env.Payload, &decoded); err != nil {
		return
	}
	resp, ok := decoded.(singletonHandoffResponse)
	if !ok {
		return
	}

	correlationID := env.AskRequestID
	sm.handoffMu.Lock()
	ch, ok := sm.handoffWaits[correlationID]
	sm.handoffMu.Unlock()

	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// sendSystemEnvelope sends a system envelope to a target node.
func (sm *SingletonManager) sendSystemEnvelope(ctx context.Context, target NodeID, typeName string, payload any, correlationID string) error {
	encoded, err := sm.codec.Encode(payload)
	if err != nil {
		return fmt.Errorf("encode %s: %w", typeName, err)
	}

	env := Envelope{
		SenderNode:     sm.cluster.LocalNodeID(),
		TargetNode:     target,
		TypeName:       typeName,
		Payload:        encoded,
		AskRequestID:   correlationID,
		SentAtUnixNano: time.Now().UnixNano(),
	}

	return sm.cluster.SendRemote(ctx, target, env)
}

// sendHandoffResponse sends a handoff response back to the requesting node.
func (sm *SingletonManager) sendHandoffResponse(target NodeID, name string, term uint64, correlationID string, ok bool, state any, errMsg string) {
	resp := singletonHandoffResponse{
		Name:  name,
		Term:  term,
		State: state,
		OK:    ok,
		Error: errMsg,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sm.sendSystemEnvelope(ctx, target, singletonHandoffRespType, resp, correlationID)
}

// startActor handles RemoveActor (if stale entry exists) + CreateActor + registry.
// Must be called with sm.mu held.
func (sm *SingletonManager) startActor(name string, s *singletonState, initialState any) {
	// Remove stale entry if the actor was previously stopped on this node.
	sm.runtime.RemoveActor(name)

	_, err := sm.runtime.CreateActor(name, initialState, s.spec.Handler, s.spec.Options...)
	if err != nil {
		return
	}

	if sm.registry != nil {
		nodeID := sm.runtime.NodeID()
		pid, err := sm.runtime.IssuePID(nodeID, name)
		if err == nil {
			sm.registry.Register(name, pid)
		}
	}
}

// stopAndCleanup stops a singleton actor and cleans up registry state.
// Must be called with sm.mu held.
func (sm *SingletonManager) stopAndCleanup(name string, s *singletonState) {
	sm.runtime.Stop(name)
	s.phase = phaseIdle

	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}

	if sm.registry != nil {
		sm.registry.Unregister(name)
	}
}

// findPrevLeader returns the previous leader for a scope. This is
// populated by the watchLoop from LeaderEvent.PrevLeader.
func (sm *SingletonManager) findPrevLeader(scope string) NodeID {
	return sm.getPrevLeader(scope)
}

func (sm *SingletonManager) setPrevLeader(scope string, prev NodeID) {
	sm.failedMu.Lock()
	defer sm.failedMu.Unlock()
	if sm.prevLeaders == nil {
		sm.prevLeaders = make(map[string]NodeID)
	}
	sm.prevLeaders[scope] = prev
}

func (sm *SingletonManager) getPrevLeader(scope string) NodeID {
	sm.failedMu.Lock()
	defer sm.failedMu.Unlock()
	if sm.prevLeaders == nil {
		return ""
	}
	return sm.prevLeaders[scope]
}
