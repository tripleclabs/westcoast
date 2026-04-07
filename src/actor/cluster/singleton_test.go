package cluster

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

var _ = fmt.Sprintf  // used in TestSingleton_DistributesAcrossNodes
var _ atomic.Int32   // used in TestSingleton_StartsOnLeader
var _ atomic.Int64   // used in TestSingleton_TwoNodes_NoFlapping

func TestSingleton_StartsOnLeader(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	var called atomic.Int32
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		called.Add(1)
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "my-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/my-singleton")
	running := sm.Running()

	if leader == "node-1" {
		if len(running) != 1 || running[0] != "my-singleton" {
			t.Errorf("node-1 is leader but singleton not running: %v", running)
		}
	} else {
		if len(running) != 0 {
			t.Errorf("node-1 is not leader but singleton is running: %v", running)
		}
	}
}

func TestSingleton_StopsOnLeadershipLoss(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}}) // sole node = leader for everything

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "leader-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// Should be running — we're the only node.
	if len(sm.Running()) != 1 {
		t.Fatal("singleton should be running on sole node")
	}

	// Another node joins — leadership may change.
	election.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/leader-singleton")
	running := sm.Running()

	if leader != "node-1" && len(running) > 0 {
		t.Error("leadership moved but singleton still running")
	}
	if leader == "node-1" && len(running) != 1 {
		t.Error("still leader but singleton not running")
	}
}

func TestSingleton_DistributesAcrossNodes(t *testing.T) {
	members := []NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}}

	// Create elections for each node — same membership, different local IDs.
	elections := make([]*RingElection, 3)
	for i, m := range members {
		elections[i] = NewRingElection(m.ID)
		elections[i].SetMembers(members)
	}

	// All three nodes should agree on leaders, and singletons should spread.
	leaders := map[NodeID]int{}
	for i := 0; i < 20; i++ {
		scope := fmt.Sprintf("singleton/svc-%d", i)
		l1, _ := elections[0].Leader(scope)
		l2, _ := elections[1].Leader(scope)
		l3, _ := elections[2].Leader(scope)

		if l1 != l2 || l2 != l3 {
			t.Errorf("scope %s: nodes disagree on leader: %s %s %s", scope, l1, l2, l3)
		}
		leaders[l1]++
	}

	// With 20 singletons and 3 nodes, should see at least 2 nodes leading.
	if len(leaders) < 2 {
		t.Errorf("singletons not distributed: %v", leaders)
	}
	t.Logf("20 singletons across 3 nodes: %v", leaders)
}

func TestSingleton_MultipleSingletons(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "singleton-a", Handler: handler})
	sm.Register(SingletonSpec{Name: "singleton-b", Handler: handler})
	sm.Register(SingletonSpec{Name: "singleton-c", Handler: handler})

	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// Sole node — all should be running.
	running := sm.Running()
	if len(running) != 3 {
		t.Errorf("expected 3 running singletons, got %d: %v", len(running), running)
	}
}

func TestSingleton_StopCleansUp(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "cleanup-test", Handler: handler})
	sm.Start(context.Background())

	time.Sleep(100 * time.Millisecond)
	if len(sm.Running()) != 1 {
		t.Fatal("should be running")
	}

	sm.Stop()

	if len(sm.Running()) != 0 {
		t.Error("should have stopped all singletons")
	}

	// Actor should be stopped in the runtime too.
	status := rt.Status("cleanup-test")
	if status != actor.ActorStopped {
		t.Errorf("actor should be stopped, got %s", status)
	}
}

func TestSingleton_ClusterOfOne(t *testing.T) {
	// Single node: no handoff protocol needed, singleton starts immediately.
	election := NewRingElection("solo")
	election.SetMembers([]NodeMeta{{ID: "solo"}})

	rt := actor.NewRuntime(actor.WithNodeID("solo"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "solo-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(50 * time.Millisecond)

	running := sm.Running()
	if len(running) != 1 || running[0] != "solo-singleton" {
		t.Errorf("expected solo-singleton running, got %v", running)
	}
}

func TestSingleton_MemberFailed_SkipsHandoff(t *testing.T) {
	// When the previous leader fails, the new leader should start immediately
	// without attempting a handoff.
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "failover-test",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/failover-test")
	if leader != "node-1" {
		// If node-1 isn't leader, simulate node-2 failing so node-1 becomes leader.
		sm.OnMemberEvent(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		election.OnMembershipChange(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		time.Sleep(100 * time.Millisecond)
	}

	running := sm.Running()
	if len(running) != 1 {
		t.Errorf("expected singleton running after failover, got %v", running)
	}
}

func TestSingleton_RemoveActorReuse(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	// Create and stop an actor.
	_, err := rt.CreateActor("reuse-test", nil, handler)
	if err != nil {
		t.Fatal(err)
	}
	rt.Stop("reuse-test")

	// Without RemoveActor, CreateActor would fail with duplicate ID.
	err = rt.RemoveActor("reuse-test")
	if err != nil {
		t.Fatalf("RemoveActor failed: %v", err)
	}

	// Now we can reuse the ID.
	_, err = rt.CreateActor("reuse-test", "new-state", handler)
	if err != nil {
		t.Fatalf("CreateActor after RemoveActor failed: %v", err)
	}

	status := rt.Status("reuse-test")
	if status != actor.ActorRunning && status != actor.ActorStarting {
		t.Errorf("expected running/starting, got %s", status)
	}
}

func TestSingleton_RemoveActorStillRunning(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	_, err := rt.CreateActor("running-test", nil, handler)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Should fail — actor is still running.
	err = rt.RemoveActor("running-test")
	if err == nil {
		t.Fatal("expected error removing running actor")
	}
}

func TestSingleton_LeadershipBounce_RestartsSameNode(t *testing.T) {
	// Singleton starts on node-1, leadership moves away and back.
	// The singleton should restart on node-1 (testing RemoveActor path).
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "bounce-test",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(50 * time.Millisecond)

	if len(sm.Running()) != 1 {
		t.Fatal("should be running initially")
	}

	// Node-2 joins, leadership may move.
	election.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})
	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/bounce-test")
	if leader != "node-1" {
		// Leadership moved to node-2. Now node-2 fails, leadership returns.
		sm.OnMemberEvent(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		election.OnMembershipChange(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		time.Sleep(100 * time.Millisecond)

		running := sm.Running()
		if len(running) != 1 {
			t.Errorf("expected singleton to restart on node-1 after bounce, got %v", running)
		}
	}
}

func TestSingleton_TwoNodes_NoFlapping(t *testing.T) {
	// Scenario (non-clustered): node-1 boots and starts the singleton.
	// node-2 joins. Without cluster coordination, we can't prevent
	// node-2 from also starting it (no handoff protocol). But node-1
	// should NOT restart or flap — it keeps its singleton stable.

	members1 := []NodeMeta{{ID: "node-1"}}

	// Node-1 boots alone — it becomes leader for everything.
	e1 := NewRingElection("node-1")
	e1.SetMembers(members1)

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))

	var starts1 atomic.Int64
	var stops1 atomic.Int64
	handler1 := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm1 := NewSingletonManager(rt1, e1, nil, nil, nil)
	sm1.Register(SingletonSpec{
		Name:         "stable-singleton",
		Handler:      handler1,
		InitialState: "from-node-1",
		Options: []actor.ActorOption{
			actor.WithStartHook(func(_ context.Context, _ string) error {
				starts1.Add(1)
				return nil
			}),
			actor.WithStopHook(func(_ context.Context, _ string) error {
				stops1.Add(1)
				return nil
			}),
		},
	})
	sm1.Start(context.Background())
	defer sm1.Stop()

	time.Sleep(100 * time.Millisecond)

	// Verify node-1 is running the singleton.
	if len(sm1.Running()) != 1 {
		t.Fatal("node-1 should be running the singleton as sole node")
	}
	if starts1.Load() != 1 {
		t.Fatalf("expected exactly 1 start on node-1, got %d", starts1.Load())
	}

	// Node-2 boots with knowledge of both nodes.
	e2 := NewRingElection("node-2")
	e2.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))

	var starts2 atomic.Int64
	handler2 := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm2 := NewSingletonManager(rt2, e2, nil, nil, nil)
	sm2.Register(SingletonSpec{
		Name:    "stable-singleton",
		Handler: handler2,
		Options: []actor.ActorOption{
			actor.WithStartHook(func(_ context.Context, _ string) error {
				starts2.Add(1)
				return nil
			}),
		},
	})
	sm2.Start(context.Background())
	defer sm2.Stop()

	// Now tell node-1's election about node-2 joining.
	e1.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})

	time.Sleep(200 * time.Millisecond)

	// Node-1 should still be running the singleton — no flapping.
	running1 := sm1.Running()
	if len(running1) != 1 || running1[0] != "stable-singleton" {
		t.Errorf("node-1 should still be running the singleton: %v", running1)
	}
	if starts1.Load() != 1 {
		t.Errorf("node-1 should have started exactly once: starts=%d", starts1.Load())
	}
	if stops1.Load() != 0 {
		t.Errorf("node-1 should NOT have been stopped: stops=%d", stops1.Load())
	}

	// Stability check.
	time.Sleep(200 * time.Millisecond)
	if starts1.Load() != 1 || stops1.Load() != 0 {
		t.Errorf("flapping: starts=%d stops=%d", starts1.Load(), stops1.Load())
	}
}

