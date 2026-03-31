package cluster

import (
	"fmt"
	"testing"
	"time"
)

func TestRingElection_DeterministicLeader(t *testing.T) {
	e1 := NewRingElection("node-1")
	e2 := NewRingElection("node-2")
	e3 := NewRingElection("node-3")

	members := []NodeMeta{
		{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"},
	}
	e1.SetMembers(members)
	e2.SetMembers(members)
	e3.SetMembers(members)

	// All nodes should agree on the leader for the same scope.
	l1, _ := e1.Leader("supervisor")
	l2, _ := e2.Leader("supervisor")
	l3, _ := e3.Leader("supervisor")

	if l1 != l2 || l2 != l3 {
		t.Errorf("should agree: e1=%s, e2=%s, e3=%s", l1, l2, l3)
	}
	t.Logf("leader for 'supervisor': %s", l1)
}

func TestRingElection_DifferentScopes(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	// Different scopes may elect different leaders.
	leaders := map[NodeID]bool{}
	for i := 0; i < 20; i++ {
		scope := fmt.Sprintf("scope-%d", i)
		l, ok := e.Leader(scope)
		if !ok {
			t.Fatalf("no leader for %s", scope)
		}
		leaders[l] = true
	}

	// With 20 scopes and 3 nodes, we should see at least 2 different leaders.
	if len(leaders) < 2 {
		t.Errorf("expected multiple leaders across scopes, got %d", len(leaders))
	}
	t.Logf("unique leaders across 20 scopes: %d", len(leaders))
}

func TestRingElection_IsLeader(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}})

	// With only one node, it must be the leader for everything.
	if !e.IsLeader("anything") {
		t.Error("sole node should be leader")
	}
}

func TestRingElection_LeaderChangesOnFailure(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	leader1, _ := e.Leader("svc")
	term1 := e.Term("svc")

	// Remove the leader.
	e.OnMembershipChange(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: leader1},
	})

	leader2, _ := e.Leader("svc")
	term2 := e.Term("svc")

	if leader2 == leader1 {
		t.Error("leader should change after failure")
	}
	if term2 <= term1 {
		t.Errorf("term should increase: was %d, now %d", term1, term2)
	}
}

func TestRingElection_TermIncreases(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	_ = e.Term("scope") // initializes scope

	// Adding a node may or may not change the leader, but if it does, term increases.
	termBefore := e.Term("scope")

	e.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-3"},
	})
	e.OnMembershipChange(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-2"},
	})

	termAfter := e.Term("scope")
	if termAfter < termBefore {
		t.Errorf("term should never decrease: was %d, now %d", termBefore, termAfter)
	}
}

func TestRingElection_Watch(t *testing.T) {
	e := NewRingElection("node-1")
	e.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	ch := e.Watch("svc")

	// Should get initial state.
	select {
	case ev := <-ch:
		if ev.Leader == "" {
			t.Error("initial event should have a leader")
		}
		t.Logf("initial: leader=%s term=%d", ev.Leader, ev.Term)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("should receive initial event")
	}

	// Trigger a change.
	leader, _ := e.Leader("svc")
	e.OnMembershipChange(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: leader},
	})

	select {
	case ev := <-ch:
		if ev.PrevLeader != leader {
			t.Errorf("prev leader should be %s, got %s", leader, ev.PrevLeader)
		}
		if ev.Leader == leader {
			t.Error("new leader should differ from failed node")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("should receive change event")
	}
}

func TestRingElection_SoleNodeAlwaysLeader(t *testing.T) {
	e := NewRingElection("solo")

	l, ok := e.Leader("scope")
	if !ok || l != "solo" {
		t.Errorf("sole node should be leader: got %s, ok=%v", l, ok)
	}
}
