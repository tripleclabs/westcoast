package crdt

import (
	"sync"
	"testing"
	"time"
)

func TestORSet_AddAndLookup(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	if err := s.Add("k1", "v1"); err != nil {
		t.Fatal(err)
	}
	e := s.Lookup("k1")
	if e == nil || e.Value != "v1" {
		t.Fatalf("got %v", e)
	}
}

func TestORSet_AddDuplicate(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Add("k", "v1")
	if err := s.Add("k", "v2"); err == nil {
		t.Error("should reject duplicate key")
	}
}

func TestORSet_Put_InsertAndReplace(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Put("k", "v1")
	if e := s.Lookup("k"); e.Value != "v1" {
		t.Fatalf("expected v1, got %v", e.Value)
	}
	s.Put("k", "v2")
	if e := s.Lookup("k"); e.Value != "v2" {
		t.Fatalf("expected v2, got %v", e.Value)
	}
	// Old entry should be tombstoned.
	if s.TombstoneCount() != 1 {
		t.Errorf("expected 1 tombstone, got %d", s.TombstoneCount())
	}
}

func TestORSet_Remove(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Add("k", "v")
	removed := s.Remove("k")
	if removed == nil || removed.Value != "v" {
		t.Fatalf("got %v", removed)
	}
	if s.Lookup("k") != nil {
		t.Error("should be gone")
	}
	if s.TombstoneCount() != 1 {
		t.Error("should have 1 tombstone")
	}
}

func TestORSet_RemoveNotFound(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	if s.Remove("nope") != nil {
		t.Error("should return nil")
	}
}

func TestORSet_RemoveIf(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Add("a", "keep")
	s.Add("b", "drop")
	s.Add("c", "drop")

	removed := s.RemoveIf(func(e Entry) bool {
		return e.Value == "drop"
	})
	if len(removed) != 2 {
		t.Errorf("expected 2 removed, got %d", len(removed))
	}
	if s.Size() != 1 {
		t.Errorf("expected 1 remaining, got %d", s.Size())
	}
	if s.Lookup("a") == nil {
		t.Error("'a' should survive")
	}
}

func TestORSet_Filter(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Add("a", "x")
	s.Add("b", "y")
	s.Add("c", "x")

	matches := s.Filter(func(e Entry) bool {
		return e.Value == "x"
	})
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestORSet_Keys(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	s.Add("a", "1")
	s.Add("b", "2")
	if len(s.Keys()) != 2 {
		t.Errorf("expected 2 keys, got %d", len(s.Keys()))
	}
}

// --- Digest-based sync ---

func TestORSet_DigestSync_TwoNodes(t *testing.T) {
	s1 := NewORSet(ORSetConfig{NodeID: "n1"})
	s2 := NewORSet(ORSetConfig{NodeID: "n2"})

	s1.Add("a", "v1")
	s2.Add("b", "v2")

	syncPair(s1, s2)

	if s1.Lookup("b") == nil {
		t.Error("s1 missing b")
	}
	if s2.Lookup("a") == nil {
		t.Error("s2 missing a")
	}
}

func TestORSet_DigestSync_ThreeNodes(t *testing.T) {
	sets := []*ORSet{
		NewORSet(ORSetConfig{NodeID: "n1"}),
		NewORSet(ORSetConfig{NodeID: "n2"}),
		NewORSet(ORSetConfig{NodeID: "n3"}),
	}
	sets[0].Add("s1", "p1")
	sets[1].Add("s2", "p2")
	sets[2].Add("s3", "p3")

	// Two full-mesh rounds is enough for 3 nodes.
	for round := 0; round < 2; round++ {
		for i := range sets {
			for j := range sets {
				if i != j {
					d := sets[i].Digest()
					delta := sets[j].DeltaFor(d)
					sets[i].MergeDelta(delta)
				}
			}
		}
	}

	for i, s := range sets {
		for _, key := range []string{"s1", "s2", "s3"} {
			if s.Lookup(key) == nil {
				t.Errorf("set[%d] missing %s", i, key)
			}
		}
	}
}

func TestORSet_DigestSync_RemovePropagates(t *testing.T) {
	s1 := NewORSet(ORSetConfig{NodeID: "n1"})
	s2 := NewORSet(ORSetConfig{NodeID: "n2"})

	s1.Add("k", "v")
	syncPair(s1, s2)

	s1.Remove("k")
	syncPair(s1, s2)

	if s1.Lookup("k") != nil {
		t.Error("s1 should not have k")
	}
	if s2.Lookup("k") != nil {
		t.Error("s2 should not have k after sync")
	}
}

func TestORSet_DigestSync_ConcurrentAddSameKey(t *testing.T) {
	s1 := NewORSet(ORSetConfig{NodeID: "n1"})
	s2 := NewORSet(ORSetConfig{NodeID: "n2"})

	s1.Add("leader", "pid-1")
	s2.Add("leader", "pid-2")

	syncPair(s1, s2)

	e1 := s1.Lookup("leader")
	e2 := s2.Lookup("leader")
	if e1 == nil || e2 == nil {
		t.Fatal("both should have leader")
	}
	if e1.Tag != e2.Tag {
		t.Errorf("should converge: s1 tag=%s, s2 tag=%s", e1.Tag, e2.Tag)
	}
}

func TestORSet_DigestSync_Noop(t *testing.T) {
	s1 := NewORSet(ORSetConfig{NodeID: "n1"})
	s2 := NewORSet(ORSetConfig{NodeID: "n2"})

	s1.Add("k", "v")
	syncPair(s1, s2)

	d := s1.Digest()
	delta := s2.DeltaFor(d)
	if len(delta.Entries) != 0 || len(delta.Tombstones) != 0 {
		t.Error("should be empty when synced")
	}
}

// --- Tombstone compaction ---

func TestORSet_Compact(t *testing.T) {
	now := time.Now()
	s := NewORSet(ORSetConfig{
		NodeID:       "n1",
		TombstoneTTL: 1 * time.Second,
		Now:          func() time.Time { return now },
	})

	s.Add("k", "v")
	s.Remove("k")

	if s.Compact() != 0 {
		t.Error("should not compact before TTL")
	}

	now = now.Add(2 * time.Second)
	if n := s.Compact(); n != 1 {
		t.Errorf("expected 1 compacted, got %d", n)
	}
	if s.TombstoneCount() != 0 {
		t.Error("should have 0 tombstones")
	}
}

// --- Concurrency ---

func TestORSet_ConcurrentAccess(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "n1"})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			switch i % 4 {
			case 0:
				s.Add("k", "v")
			case 1:
				s.Remove("k")
			case 2:
				s.Lookup("k")
			case 3:
				s.Digest()
			}
		}(i)
	}
	wg.Wait()
}

// --- Vector clock origin ---

func TestORSet_TimestampTracksOrigin(t *testing.T) {
	s := NewORSet(ORSetConfig{NodeID: "node-2"})
	s.Add("svc", "pid")

	e := s.Lookup("svc")
	if e == nil {
		t.Fatal("missing entry")
	}
	// The vector clock should show node-2 performed this write.
	if e.Timestamp["node-2"] == 0 {
		t.Error("timestamp should reflect originating node")
	}
}

func syncPair(a, b *ORSet) {
	dA := a.Digest()
	deltaBA := b.DeltaFor(dA)
	a.MergeDelta(deltaBA)

	dB := b.Digest()
	deltaAB := a.DeltaFor(dB)
	b.MergeDelta(deltaAB)
}
