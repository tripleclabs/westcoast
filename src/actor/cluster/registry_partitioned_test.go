package cluster

import (
	"errors"
	"fmt"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestPartitioned_RegisterOnHomeNode(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1", "node-2", "node-3")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	// Find a name that hashes to node-1.
	var homeName string
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("svc-%d", i)
		if r.HomeNode(name) == "node-1" {
			homeName = name
			break
		}
	}
	if homeName == "" {
		t.Fatal("could not find a name homed on node-1")
	}

	p := actor.PID{Namespace: "node-1", ActorID: "actor-a", Generation: 1}
	if err := r.Register(homeName, p); err != nil {
		t.Fatalf("register on home node should succeed: %v", err)
	}

	got, ok := r.Lookup(homeName)
	if !ok {
		t.Fatal("should find registered name")
	}
	if got.ActorID != "actor-a" {
		t.Errorf("got %s", got.ActorID)
	}
}

func TestPartitioned_RejectNonHomeNode(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1", "node-2", "node-3")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	// Find a name NOT homed on node-1.
	var remoteName string
	for i := 0; i < 1000; i++ {
		name := fmt.Sprintf("svc-%d", i)
		if r.HomeNode(name) != "node-1" {
			remoteName = name
			break
		}
	}
	if remoteName == "" {
		t.Fatal("could not find a name not homed on node-1")
	}

	p := actor.PID{Namespace: "node-2", ActorID: "actor-b", Generation: 1}
	err := r.Register(remoteName, p)
	if err == nil {
		t.Fatal("should reject registration on non-home node")
	}
	if !errors.Is(err, ErrNotHomeNode) {
		t.Errorf("expected ErrNotHomeNode, got %v", err)
	}
}

func TestPartitioned_Idempotent(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	p := actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1}
	r.Register("svc", p)
	if err := r.Register("svc", p); err != nil {
		t.Errorf("idempotent register should not error: %v", err)
	}
}

func TestPartitioned_Conflict(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	r.Register("svc", actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1})
	err := r.Register("svc", actor.PID{Namespace: "node-1", ActorID: "b", Generation: 1})
	if err == nil {
		t.Error("should reject conflicting registration")
	}
}

func TestPartitioned_Unregister(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	p := actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1}
	r.Register("svc", p)

	got, ok := r.Unregister("svc")
	if !ok {
		t.Fatal("should unregister")
	}
	if got.ActorID != "a" {
		t.Errorf("got %s", got.ActorID)
	}
	if _, ok := r.Lookup("svc"); ok {
		t.Error("should be gone")
	}
}

func TestPartitioned_UnregisterByNode(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1")

	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(members)

	r.Register("svc-a", actor.PID{Namespace: "node-2", ActorID: "a", Generation: 1})
	r.Register("svc-b", actor.PID{Namespace: "node-2", ActorID: "b", Generation: 1})
	r.Register("svc-c", actor.PID{Namespace: "node-1", ActorID: "c", Generation: 1})

	removed := r.UnregisterByNode("node-2")
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
	if _, ok := r.Lookup("svc-c"); !ok {
		t.Error("svc-c should survive")
	}
}

func TestPartitioned_RebalanceOnJoin(t *testing.T) {
	topo := NewRingTopology(2, 10)

	// Start with just node-1 — it owns everything.
	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(makeMembers("node-1"))

	// Register several names.
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("svc-%d", i)
		r.Register(name, actor.PID{Namespace: "node-1", ActorID: name, Generation: 1})
	}

	before := len(r.AllEntries())
	if before != 20 {
		t.Fatalf("expected 20 entries, got %d", before)
	}

	// Node-2 joins — some names should migrate away from node-1.
	r.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2", Addr: "10.0.0.2:9000"},
	})

	after := len(r.AllEntries())
	if after >= before {
		t.Errorf("expected some entries to move off node-1: before=%d, after=%d", before, after)
	}
	t.Logf("rebalance: %d → %d entries on node-1 after node-2 joined", before, after)
}

func TestPartitioned_RebalanceOnLeave(t *testing.T) {
	topo := NewRingTopology(2, 10)

	// Start with two nodes.
	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(makeMembers("node-1", "node-2"))

	// Register names that are homed on node-1.
	registered := 0
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("svc-%d", i)
		if err := r.Register(name, actor.PID{Namespace: "node-1", ActorID: name, Generation: 1}); err == nil {
			registered++
		}
	}
	before := len(r.AllEntries())

	// Node-2 leaves — node-1 now owns everything, but we only keep what
	// was already registered locally.
	r.OnMembershipChange(MemberEvent{
		Type:   MemberLeave,
		Member: NodeMeta{ID: "node-2"},
	})

	after := len(r.AllEntries())
	// All entries should still be here (they were on node-1 already),
	// and potentially more names are now homed on node-1.
	if after < before {
		t.Errorf("should not lose entries on node leave: before=%d, after=%d", before, after)
	}
	t.Logf("node-2 left: %d → %d entries on node-1", before, after)
}

func TestPartitioned_HomeNodeDeterministic(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("node-1", "node-2", "node-3")

	r1 := NewPartitionedRegistry("node-1", topo)
	r1.SetMembers(members)

	r2 := NewPartitionedRegistry("node-2", topo)
	r2.SetMembers(members)

	// Both nodes should agree on the home node for any given name.
	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("svc-%d", i)
		h1 := r1.HomeNode(name)
		h2 := r2.HomeNode(name)
		if h1 != h2 {
			t.Errorf("name %s: node-1 says %s, node-2 says %s", name, h1, h2)
		}
	}
}

func TestPartitioned_AllEntries(t *testing.T) {
	topo := NewRingTopology(2, 10)
	r := NewPartitionedRegistry("node-1", topo)
	r.SetMembers(makeMembers("node-1"))

	r.Register("a", actor.PID{Namespace: "node-1", ActorID: "x", Generation: 1})
	r.Register("b", actor.PID{Namespace: "node-1", ActorID: "y", Generation: 1})

	all := r.AllEntries()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}
