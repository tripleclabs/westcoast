package cluster

import (
	"context"
	"sync"

	"github.com/tripleclabs/westcoast/src/actor"
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
}

// SingletonManager ensures that a set of singleton actors run on
// exactly one node — the current leader for each singleton's scope.
// When leadership changes, the actor stops on the old leader and
// starts on the new one.
type SingletonManager struct {
	runtime  *actor.Runtime
	election *RingElection
	registry *CRDTRegistry
	cluster  *Cluster

	mu         sync.Mutex
	singletons map[string]*singletonState
	cancel     context.CancelFunc
}

type singletonState struct {
	spec    SingletonSpec
	running bool
}

// NewSingletonManager creates a SingletonManager. The cluster parameter
// is optional — only needed when using Placement predicates on specs.
func NewSingletonManager(runtime *actor.Runtime, election *RingElection, registry *CRDTRegistry, cluster ...*Cluster) *SingletonManager {
	sm := &SingletonManager{
		runtime:    runtime,
		election:   election,
		registry:   registry,
		singletons: make(map[string]*singletonState),
	}
	if len(cluster) > 0 {
		sm.cluster = cluster[0]
	}
	return sm
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

// Stop terminates all managed singletons and stops watching.
func (sm *SingletonManager) Stop() {
	if sm.cancel != nil {
		sm.cancel()
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	for name, s := range sm.singletons {
		if s.running {
			sm.runtime.Stop(name)
			s.running = false
		}
	}
}

// Running returns the names of singletons currently running on this node.
func (sm *SingletonManager) Running() []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	var out []string
	for name, s := range sm.singletons {
		if s.running {
			out = append(out, name)
		}
	}
	return out
}

func (sm *SingletonManager) watchLoop(ctx context.Context, name string, ch <-chan LeaderEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
			sm.reconcile()
		}
	}
}

// reconcile checks each singleton: start if we're the leader (among
// placement-eligible nodes), stop if we're not.
func (sm *SingletonManager) reconcile() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	nodeID := sm.runtime.NodeID()

	for name, s := range sm.singletons {
		scope := "singleton/" + name
		var isLeader bool

		if s.spec.Placement != nil && sm.cluster != nil {
			// Placement-constrained: only matching nodes are candidates.
			members := sm.cluster.Members()
			members = append(members, sm.cluster.Self())
			leader, ok := sm.election.LeaderAmong(scope, s.spec.Placement, members)
			isLeader = ok && string(leader) == nodeID
		} else {
			isLeader = sm.election.IsLeader(scope)
		}

		if isLeader && !s.running {
			// Start the singleton on this node.
			_, err := sm.runtime.CreateActor(name, s.spec.InitialState, s.spec.Handler, s.spec.Options...)
			if err != nil {
				continue // actor may already exist from a previous run
			}
			s.running = true

			// Register the name so other nodes can find it.
			if sm.registry != nil {
				pid, err := sm.runtime.IssuePID(nodeID, name)
				if err == nil {
					sm.registry.Register(name, pid)
				}
			}
		}

		if !isLeader && s.running {
			// Stop the singleton — leadership moved elsewhere.
			sm.runtime.Stop(name)
			s.running = false

			if sm.registry != nil {
				sm.registry.Unregister(name)
			}
		}
	}
}
