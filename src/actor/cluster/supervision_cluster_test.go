package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestClusterSupervisor_LeaderHandlesFailure(t *testing.T) {
	ctx := context.Background()

	// Set up election with 3 nodes.
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	// Set up a minimal cluster (provider only, no transport needed for this test).
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(ctx)
	defer c.Stop()

	// Register actors on node-3 in the CRDT registry.
	registry := NewCRDTRegistry("node-1")
	registry.Register("svc-a", actor.PID{Namespace: "node-3", ActorID: "a", Generation: 1})
	registry.Register("svc-b", actor.PID{Namespace: "node-3", ActorID: "b", Generation: 1})
	registry.Register("svc-c", actor.PID{Namespace: "node-1", ActorID: "c", Generation: 1})

	// Track which node is the supervisor leader.
	leader, _ := election.Leader(clusterSupervisorScope)

	// Create supervisor — only run it on the leader node.
	if leader != "node-1" {
		// For this test, force leadership by removing other nodes.
		election.OnMembershipChange(MemberEvent{Type: MemberFailed, Member: NodeMeta{ID: leader}})
		leader, _ = election.Leader(clusterSupervisorScope)
		if leader != "node-1" {
			t.Skipf("cannot force leadership to node-1, leader is %s", leader)
		}
	}

	policy := SimpleRestartPolicy{}
	supervisor := NewClusterSupervisor(ClusterSupervisorConfig{
		Election: election,
		Cluster:  c,
		Registry: registry,
		Policy:   policy,
	})
	supervisor.Start(ctx)
	defer supervisor.Stop()

	// Add node-2 to the cluster so there's a target for restarts.
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:19999"})
	time.Sleep(50 * time.Millisecond)

	// Simulate node-3 failure.
	provider.AddMember(NodeMeta{ID: "node-3", Addr: "127.0.0.1:19998"})
	time.Sleep(50 * time.Millisecond)

	// Trigger the failure event manually through the cluster's OnMemberEvent.
	c.cfg.OnMemberEvent(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-3"},
	})

	time.Sleep(100 * time.Millisecond)

	decisions := supervisor.Decisions()
	if len(decisions) != 2 {
		t.Fatalf("expected 2 placement decisions (svc-a and svc-b), got %d: %+v", len(decisions), decisions)
	}

	// All decisions should target live nodes.
	for _, d := range decisions {
		if d.TargetNode == "node-3" {
			t.Error("should not place on failed node")
		}
		if d.Action != PlacementRestart {
			t.Error("should restart")
		}
	}

	// Registry should be cleaned up.
	if _, ok := registry.Lookup("svc-a"); ok {
		t.Error("svc-a should be unregistered from failed node")
	}
	// svc-c should survive (it's on node-1).
	if _, ok := registry.Lookup("svc-c"); !ok {
		t.Error("svc-c should still exist")
	}
}

func TestClusterSupervisor_NonLeaderIgnores(t *testing.T) {
	ctx := context.Background()

	election := NewRingElection("node-2")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-2")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(ctx)
	defer c.Stop()

	registry := NewCRDTRegistry("node-2")
	registry.Register("svc-a", actor.PID{Namespace: "node-3", ActorID: "a", Generation: 1})

	leader, _ := election.Leader(clusterSupervisorScope)

	supervisor := NewClusterSupervisor(ClusterSupervisorConfig{
		Election: election,
		Cluster:  c,
		Registry: registry,
		Policy:   SimpleRestartPolicy{},
	})
	supervisor.Start(ctx)
	defer supervisor.Stop()

	// Simulate node-3 failure.
	c.cfg.OnMemberEvent(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-3"},
	})

	time.Sleep(100 * time.Millisecond)

	decisions := supervisor.Decisions()
	if leader == "node-2" {
		// If we happen to be the leader, we'll have decisions.
		t.Logf("node-2 IS the leader — got %d decisions", len(decisions))
	} else {
		// Non-leader should have no decisions.
		if len(decisions) != 0 {
			t.Errorf("non-leader should not make decisions, got %d", len(decisions))
		}
	}
}

func TestClusterSupervisor_SimpleRestartPolicy(t *testing.T) {
	policy := SimpleRestartPolicy{}

	live := []NodeMeta{{ID: "node-1"}, {ID: "node-2"}}
	decisions := policy.OnNodeFailed("node-3", []string{"svc-a", "svc-b", "svc-c"}, live)

	if len(decisions) != 3 {
		t.Fatalf("expected 3 decisions, got %d", len(decisions))
	}

	// Round-robin across live nodes.
	targets := map[NodeID]int{}
	for _, d := range decisions {
		targets[d.TargetNode]++
		if d.Action != PlacementRestart {
			t.Error("should restart")
		}
	}

	if targets["node-1"] == 0 || targets["node-2"] == 0 {
		t.Errorf("should distribute across nodes: %v", targets)
	}
}

func TestClusterSupervisor_NoActorsNoDecisions(t *testing.T) {
	policy := SimpleRestartPolicy{}
	decisions := policy.OnNodeFailed("node-3", nil, []NodeMeta{{ID: "node-1"}})
	if len(decisions) != 0 {
		t.Error("no actors should mean no decisions")
	}
}
