package cluster

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

var _ = fmt.Sprintf // used in TestSingleton_DistributesAcrossNodes
var _ atomic.Int32  // used in TestSingleton_StartsOnLeader

func TestSingleton_StartsOnLeader(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	var called atomic.Int32
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		called.Add(1)
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil)
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

	sm := NewSingletonManager(rt, election, nil)
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

	sm := NewSingletonManager(rt, election, nil)
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

	sm := NewSingletonManager(rt, election, nil)
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
