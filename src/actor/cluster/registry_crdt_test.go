package cluster

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func pid(ns, id string, gen uint64) actor.PID {
	return actor.PID{Namespace: ns, ActorID: id, Generation: gen}
}

func TestCRDT_RegisterAndLookup(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
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

func TestCRDT_RegisterIdempotent(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	p := pid("node-1", "a", 1)
	reg.Register("svc", p)
	if err := reg.Register("svc", p); err != nil {
		t.Errorf("idempotent re-register should not error: %v", err)
	}
}

func TestCRDT_RegisterConflict(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	reg.Register("svc", pid("node-1", "a", 1))
	if err := reg.Register("svc", pid("node-1", "b", 1)); err == nil {
		t.Error("should reject conflicting registration")
	}
}

func TestCRDT_Unregister(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
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

func TestCRDT_UnregisterByNode(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
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

func TestCRDT_NamesByNode(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	reg.Register("svc-a", pid("node-2", "a", 1))
	reg.Register("svc-b", pid("node-2", "b", 1))
	names := reg.NamesByNode("node-2")
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

func TestCRDT_DigestSync_TwoNodes(t *testing.T) {
	r1 := NewCRDTRegistry("node-1")
	r2 := NewCRDTRegistry("node-2")

	r1.Register("svc-a", pid("node-1", "a", 1))
	r2.Register("svc-b", pid("node-2", "b", 1))

	// Bidirectional sync.
	d1 := r1.Digest()
	delta21 := r2.DeltaFor(d1) // what r2 has that r1 doesn't
	r1.MergeDelta(delta21)

	d2 := r2.Digest()
	delta12 := r1.DeltaFor(d2)
	r2.MergeDelta(delta12)

	if _, ok := r1.Lookup("svc-b"); !ok {
		t.Error("r1 should see svc-b")
	}
	if _, ok := r2.Lookup("svc-a"); !ok {
		t.Error("r2 should see svc-a")
	}
}

func TestCRDT_DigestSync_ThreeNodes_Converge(t *testing.T) {
	regs := []*CRDTRegistry{
		NewCRDTRegistry("node-1"),
		NewCRDTRegistry("node-2"),
		NewCRDTRegistry("node-3"),
	}

	regs[0].Register("svc-1", pid("node-1", "a", 1))
	regs[1].Register("svc-2", pid("node-2", "b", 1))
	regs[2].Register("svc-3", pid("node-3", "c", 1))

	// Run digest-based gossip rounds.
	for round := 0; round < 3; round++ {
		for i := range regs {
			for j := range regs {
				if i == j {
					continue
				}
				digest := regs[i].Digest()
				delta := regs[j].DeltaFor(digest)
				regs[i].MergeDelta(delta)
			}
		}
	}

	for i, reg := range regs {
		for _, svc := range []string{"svc-1", "svc-2", "svc-3"} {
			if _, ok := reg.Lookup(svc); !ok {
				t.Errorf("reg[%d] missing %s", i, svc)
			}
		}
	}
}

func TestCRDT_DigestSync_RemovePropagates(t *testing.T) {
	r1 := NewCRDTRegistry("node-1")
	r2 := NewCRDTRegistry("node-2")

	r1.Register("svc", pid("node-1", "a", 1))
	syncRegistries(r1, r2)

	if _, ok := r2.Lookup("svc"); !ok {
		t.Fatal("r2 should have svc after sync")
	}

	r1.Unregister("svc")
	syncRegistries(r1, r2)

	if _, ok := r2.Lookup("svc"); ok {
		t.Error("r2 should not have svc after remove sync")
	}
}

func TestCRDT_DigestSync_ConcurrentRegister(t *testing.T) {
	r1 := NewCRDTRegistry("node-1")
	r2 := NewCRDTRegistry("node-2")

	r1.Register("leader", pid("node-1", "a", 1))
	r2.Register("leader", pid("node-2", "b", 1))

	syncRegistries(r1, r2)

	p1, _ := r1.Lookup("leader")
	p2, _ := r2.Lookup("leader")
	if p1.Key() != p2.Key() {
		t.Errorf("should converge: r1=%s, r2=%s", p1.Key(), p2.Key())
	}
}

func TestCRDT_DigestSync_NoopWhenSynced(t *testing.T) {
	r1 := NewCRDTRegistry("node-1")
	r2 := NewCRDTRegistry("node-2")

	r1.Register("k", pid("node-1", "a", 1))
	syncRegistries(r1, r2)

	d := r1.Digest()
	delta := r2.DeltaFor(d)
	if len(delta.Entries) != 0 || len(delta.Tombstones) != 0 {
		t.Error("should produce empty delta when synced")
	}
}

func TestCRDT_OnMembershipChange_NodeFailed(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	reg.Register("svc", pid("node-2", "a", 1))
	reg.OnMembershipChange(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-2"},
	})
	if _, ok := reg.Lookup("svc"); ok {
		t.Error("should unregister names from failed node")
	}
}

func TestCRDT_AllEntries(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	reg.Register("a", pid("node-1", "x", 1))
	reg.Register("b", pid("node-1", "y", 1))
	all := reg.AllEntries()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

func TestCRDT_Compact(t *testing.T) {
	reg := NewCRDTRegistry("node-1")
	reg.Register("svc", pid("node-1", "a", 1))
	reg.Unregister("svc")

	// Compact won't remove fresh tombstones.
	n := reg.Compact()
	if n != 0 {
		t.Errorf("should not compact fresh tombstones, got %d", n)
	}
}

func syncRegistries(a, b *CRDTRegistry) {
	dA := a.Digest()
	deltaBA := b.DeltaFor(dA)
	a.MergeDelta(deltaBA)

	dB := b.Digest()
	deltaAB := a.DeltaFor(dB)
	b.MergeDelta(deltaAB)
}
