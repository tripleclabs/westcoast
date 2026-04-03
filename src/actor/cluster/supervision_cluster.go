package cluster

import (
	"context"
	"sync"
)

const clusterSupervisorScope = "cluster_supervisor"

// ClusterSupervisor watches for node failures and coordinates actor
// restarts across the cluster. It uses leader election to ensure only
// one node makes placement decisions at a time, preventing duplicate
// restarts.
//
// This is a user-space component — the platform provides the building
// blocks (election, membership events, registry queries), and the
// supervisor composes them.
type ClusterSupervisor struct {
	election LeaderElection
	cluster  *Cluster
	registry *DistributedRegistry // for looking up actor names by node
	policy   ClusterSupervisionPolicy
	scope    string

	mu        sync.Mutex
	decisions []PlacementDecision // accumulated decisions for inspection/testing
	cancel    context.CancelFunc
}

// ClusterSupervisorConfig configures a cluster supervisor.
type ClusterSupervisorConfig struct {
	Election LeaderElection
	Cluster  *Cluster
	Registry *DistributedRegistry
	Policy   ClusterSupervisionPolicy
	// Scope for the leader election. Defaults to "cluster_supervisor".
	Scope string
}

func NewClusterSupervisor(cfg ClusterSupervisorConfig) *ClusterSupervisor {
	scope := cfg.Scope
	if scope == "" {
		scope = clusterSupervisorScope
	}
	return &ClusterSupervisor{
		election: cfg.Election,
		cluster:  cfg.Cluster,
		registry: cfg.Registry,
		policy:   cfg.Policy,
		scope:    scope,
	}
}

// Start begins watching for membership events. The supervisor subscribes
// to the cluster's membership event callback.
func (cs *ClusterSupervisor) Start(ctx context.Context) {
	ctx, cs.cancel = context.WithCancel(ctx)

	// Wire into the cluster's membership event stream via the thread-safe setter.
	cs.cluster.SetOnMemberEvent(func(ev MemberEvent) {
		cs.election.OnMembershipChange(ev)
		if ev.Type == MemberFailed {
			cs.handleNodeFailure(ev.Member)
		}
	})
}

// Stop terminates the supervisor.
func (cs *ClusterSupervisor) Stop() {
	if cs.cancel != nil {
		cs.cancel()
	}
}

// Decisions returns all placement decisions made so far. For testing/inspection.
func (cs *ClusterSupervisor) Decisions() []PlacementDecision {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	out := make([]PlacementDecision, len(cs.decisions))
	copy(out, cs.decisions)
	return out
}

func (cs *ClusterSupervisor) handleNodeFailure(failed NodeMeta) {
	// Only the leader handles failures.
	if !cs.election.IsLeader(cs.scope) {
		return
	}

	term := cs.election.Term(cs.scope)

	// Query the registry for actors that were on the failed node.
	actorNames := cs.registry.NamesByNode(failed.ID)
	if len(actorNames) == 0 {
		return
	}

	// Get live members (excluding the failed node).
	members := cs.cluster.Members()
	var live []NodeMeta
	for _, m := range members {
		if m.ID != failed.ID {
			live = append(live, m)
		}
	}

	// Ask the policy for placement decisions.
	decisions := cs.policy.OnNodeFailed(failed.ID, actorNames, live)

	// Verify we're still the leader before committing (fencing check).
	if cs.election.Term(cs.scope) != term {
		return // leadership changed during processing — abort
	}

	// Record committed decisions.
	cs.mu.Lock()
	cs.decisions = append(cs.decisions, decisions...)
	cs.mu.Unlock()

	// Clean up the failed node's names from the registry.
	cs.registry.UnregisterByNode(failed.ID)
}
