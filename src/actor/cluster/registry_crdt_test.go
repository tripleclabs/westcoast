package cluster

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func pid(ns, id string, gen uint64) actor.PID {
	return actor.PID{Namespace: ns, ActorID: id, Generation: gen}
}

func TestRegistry_RegisterAndLookup(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	if err := reg.Register("svc-a", pid("node-1", "actor-a", 1)); err != nil {
		t.Fatal(err)
	}
	got, ok := reg.Lookup("svc-a")
	if !ok {
		t.Fatal("should find registered name")
	}
	if got.ActorID != "actor-a" {
		t.Errorf("got %s, want actor-a", got.ActorID)
	}
}

func TestRegistry_RegisterIdempotent(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	p := pid("node-1", "a", 1)
	reg.Register("svc", p)
	if err := reg.Register("svc", p); err != nil {
		t.Errorf("idempotent re-register should not error: %v", err)
	}
}

func TestRegistry_RegisterConflict(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("svc", pid("node-1", "a", 1))
	if err := reg.Register("svc", pid("node-1", "b", 1)); err == nil {
		t.Error("should reject conflicting registration")
	}
}

func TestRegistry_Unregister(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("svc", pid("node-1", "a", 1))
	got, ok := reg.Unregister("svc")
	if !ok {
		t.Fatal("should unregister")
	}
	if got.ActorID != "a" {
		t.Errorf("got %s", got.ActorID)
	}
	if _, ok := reg.Lookup("svc"); ok {
		t.Error("should not find after unregister")
	}
}

func TestRegistry_UnregisterByNode(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("svc-a", pid("node-2", "a", 1))
	reg.Register("svc-b", pid("node-2", "b", 1))
	reg.Register("svc-c", pid("node-1", "c", 1))

	removed := reg.UnregisterByNode("node-2")
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
	if _, ok := reg.Lookup("svc-a"); ok {
		t.Error("svc-a should be gone")
	}
	if _, ok := reg.Lookup("svc-c"); !ok {
		t.Error("svc-c should still exist")
	}
}

func TestRegistry_NamesByNode(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("svc-a", pid("node-2", "a", 1))
	reg.Register("svc-b", pid("node-2", "b", 1))
	names := reg.NamesByNode("node-2")
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestRegistry_OnMembershipChange_NodeFailed(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("svc", pid("node-2", "a", 1))
	reg.OnMembershipChange(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-2"},
	})
	if _, ok := reg.Lookup("svc"); ok {
		t.Error("should unregister names from failed node")
	}
}

func TestRegistry_AllEntries(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("a", pid("node-1", "x", 1))
	reg.Register("b", pid("node-1", "y", 1))
	all := reg.AllEntries()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestRegistry_Range(t *testing.T) {
	reg := NewDistributedRegistry("node-1")
	reg.Register("a", pid("node-1", "x", 1))
	reg.Register("b", pid("node-1", "y", 1))

	var count int
	reg.Range(func(name string, _ actor.PID) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2 from Range, got %d", count)
	}
}
