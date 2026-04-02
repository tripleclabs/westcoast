package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestSingleton_PlacementOnlyMatchingNodeLeads(t *testing.T) {
	// 3 nodes: node-1 has GPUs, node-2 and node-3 don't.
	members := []NodeMeta{
		{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "4"}},
		{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "0"}},
		{ID: "node-3", Addr: "127.0.0.1:0", Tags: map[string]string{}},
	}

	e1 := NewRingElection("node-1")
	e1.SetMembers(members)

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c1, _ := NewCluster(ClusterConfig{
		Self:      members[0],
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})
	// Manually add peers so Members() returns them.
	provider.AddMember(members[1])
	provider.AddMember(members[2])
	c1.Start(context.Background())
	defer c1.Stop()
	time.Sleep(50 * time.Millisecond)

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	sm := NewSingletonManager(rt1, e1, nil, c1)
	sm.Register(SingletonSpec{
		Name:      "gpu-coordinator",
		Handler:   nop,
		Placement: TagGTE("gpus", 1),
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// node-1 has GPUs and should be the leader among eligible nodes.
	running := sm.Running()
	if len(running) != 1 || running[0] != "gpu-coordinator" {
		t.Errorf("node-1 (gpu) should run the singleton, got %v", running)
	}
}

func TestSingleton_PlacementSkipsNonMatchingLeader(t *testing.T) {
	members := []NodeMeta{
		{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "4"}},
		{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "0"}},
	}

	e2 := NewRingElection("node-2")
	e2.SetMembers(members)

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-2")
	c2, _ := NewCluster(ClusterConfig{
		Self:      members[1],
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})
	provider.AddMember(members[0])
	c2.Start(context.Background())
	defer c2.Stop()
	time.Sleep(50 * time.Millisecond)

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	sm := NewSingletonManager(rt2, e2, nil, c2)
	sm.Register(SingletonSpec{
		Name:      "gpu-coordinator",
		Handler:   nop,
		Placement: TagGTE("gpus", 1),
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// node-2 should NOT run this — it doesn't match placement.
	if len(sm.Running()) != 0 {
		t.Error("node-2 (no GPUs) should not run gpu-coordinator")
	}
}

func TestSingleton_PlacementNilIsUnrestricted(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	sm := NewSingletonManager(rt, e, nil)
	sm.Register(SingletonSpec{
		Name:    "unrestricted",
		Handler: nop,
		// Placement: nil — any node can host
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	if len(sm.Running()) != 1 {
		t.Error("nil placement should run on any node")
	}
}

func TestSingleton_PlacementBothNodesAgree(t *testing.T) {
	// Both nodes should agree that the GPU node leads.
	members := []NodeMeta{
		{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "8"}},
		{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "4"}},
		{ID: "node-3", Addr: "127.0.0.1:0", Tags: map[string]string{}},
	}

	scope := "singleton/gpu-coord"
	matcher := TagGTE("gpus", 1)

	e1 := NewRingElection("node-1")
	e1.SetMembers(members)
	e2 := NewRingElection("node-2")
	e2.SetMembers(members)

	l1, _ := e1.LeaderAmong(scope, matcher, members)
	l2, _ := e2.LeaderAmong(scope, matcher, members)

	if l1 != l2 {
		t.Errorf("nodes disagree: e1=%s, e2=%s", l1, l2)
	}

	// node-3 should never be the leader (no GPUs).
	if l1 == "node-3" {
		t.Error("node-3 has no GPUs, should not lead")
	}
	t.Logf("leader among GPU nodes: %s", l1)
}
