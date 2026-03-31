package crdt

import (
	"fmt"
	"sync"
	"time"
)

// Entry is a single element in the OR-Set.
type Entry struct {
	Key       string
	Value     any
	Tag       Tag             // unique identifier for this add operation
	Timestamp VectorTimestamp // causal timestamp — encodes who and when
}

// tombstone records a removal, kept until compaction.
type tombstone struct {
	Key       string
	Tag       Tag
	RemovedAt time.Time
}

// ORSet is an Observed-Remove Set CRDT with digest-based anti-entropy.
//
// Uniqueness is by key: each key maps to at most one live entry.
// Concurrent adds of the same key are resolved by last-writer-wins
// (vector clock), with deterministic tag tiebreak for true concurrency.
//
// Synchronization protocol:
//  1. Node A sends Digest() (key→tag map) to node B.
//  2. B calls DeltaFor(digestA) to compute what A needs.
//  3. A calls MergeDelta(delta) to apply.
//  4. Reverse direction for full sync.
//
// Tombstones are compacted after TombstoneTTL via Compact().
type ORSet struct {
	nodeID string
	clock  *vectorClock
	now    func() time.Time

	mu         sync.RWMutex
	entries    map[string]*Entry
	tombstones map[Tag]tombstone // tag → tombstone
	tagSeq     uint64

	TombstoneTTL time.Duration
}

type ORSetConfig struct {
	NodeID       string
	TombstoneTTL time.Duration
	Now          func() time.Time
}

func NewORSet(cfg ORSetConfig) *ORSet {
	if cfg.TombstoneTTL == 0 {
		cfg.TombstoneTTL = 5 * time.Minute
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &ORSet{
		nodeID:       cfg.NodeID,
		clock:        newVectorClock(cfg.NodeID),
		now:          cfg.Now,
		entries:      make(map[string]*Entry),
		tombstones:   make(map[Tag]tombstone),
		TombstoneTTL: cfg.TombstoneTTL,
	}
}

// Add inserts an entry. Fails if the key already exists — call Remove first
// to replace. This is deliberate: the registry layer decides conflict policy,
// not the CRDT.
func (s *ORSet) Add(key string, value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.entries[key]; ok {
		return fmt.Errorf("key %q already exists", key)
	}

	s.tagSeq++
	tag := NewTag(s.nodeID, s.tagSeq)
	ts := s.clock.Tick()

	s.entries[key] = &Entry{
		Key:       key,
		Value:     value,
		Tag:       tag,
		Timestamp: ts,
	}
	delete(s.tombstones, tag)
	return nil
}

// Put inserts or replaces an entry. If the key exists, the old entry is
// tombstoned and the new one takes its place.
func (s *ORSet) Put(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.entries[key]; ok {
		s.tombstones[existing.Tag] = tombstone{
			Key: key, Tag: existing.Tag, RemovedAt: s.now(),
		}
		delete(s.entries, key)
	}

	s.tagSeq++
	tag := NewTag(s.nodeID, s.tagSeq)
	ts := s.clock.Tick()

	s.entries[key] = &Entry{
		Key:       key,
		Value:     value,
		Tag:       tag,
		Timestamp: ts,
	}
}

// Remove deletes an entry by key. Returns the removed entry, or nil.
func (s *ORSet) Remove(key string) *Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[key]
	if !ok {
		return nil
	}

	delete(s.entries, key)
	s.tombstones[entry.Tag] = tombstone{
		Key: key, Tag: entry.Tag, RemovedAt: s.now(),
	}
	return entry
}

// Lookup returns the entry for a key, or nil if not found.
func (s *ORSet) Lookup(key string) *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[key]
	if !ok {
		return nil
	}
	cp := *e
	return &cp
}

// Keys returns all live keys.
func (s *ORSet) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.entries))
	for k := range s.entries {
		out = append(out, k)
	}
	return out
}

// Entries returns copies of all live entries.
func (s *ORSet) Entries() []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Entry, 0, len(s.entries))
	for _, e := range s.entries {
		out = append(out, *e)
	}
	return out
}

// RemoveIf removes all entries matching the predicate. Returns removed keys.
// The predicate receives a copy of each entry.
func (s *ORSet) RemoveIf(pred func(Entry) bool) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	var removed []string
	for key, e := range s.entries {
		if pred(*e) {
			s.tombstones[e.Tag] = tombstone{Key: key, Tag: e.Tag, RemovedAt: now}
			delete(s.entries, key)
			removed = append(removed, key)
		}
	}
	return removed
}

// Filter returns copies of all entries matching the predicate.
func (s *ORSet) Filter(pred func(Entry) bool) []Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Entry
	for _, e := range s.entries {
		if pred(*e) {
			out = append(out, *e)
		}
	}
	return out
}

// --- Digest-based anti-entropy ---

// Digest is a compact summary: key → tag for each live entry.
type Digest map[string]Tag

// Digest returns the current state summary.
func (s *ORSet) Digest() Digest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := make(Digest, len(s.entries))
	for key, e := range s.entries {
		d[key] = e.Tag
	}
	return d
}

// StateDelta contains what a peer needs to converge.
type StateDelta struct {
	Entries    []Entry
	Tombstones []tombstone
}

// DeltaFor computes what the peer (who sent peerDigest) is missing.
func (s *ORSet) DeltaFor(peerDigest Digest) StateDelta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var delta StateDelta

	// Entries the peer is missing or has stale.
	for key, entry := range s.entries {
		peerTag, peerHas := peerDigest[key]
		if !peerHas || peerTag != entry.Tag {
			delta.Entries = append(delta.Entries, *entry)
		}
	}

	// Tombstones for entries the peer has that we've removed.
	for _, ts := range s.tombstones {
		if peerTag, peerHas := peerDigest[ts.Key]; peerHas && peerTag == ts.Tag {
			delta.Tombstones = append(delta.Tombstones, ts)
		}
	}

	return delta
}

// MergeDelta applies a state delta from a peer.
func (s *ORSet) MergeDelta(delta StateDelta) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Tombstones first.
	for _, ts := range delta.Tombstones {
		existing, ok := s.entries[ts.Key]
		if ok && existing.Tag == ts.Tag {
			delete(s.entries, ts.Key)
			if _, already := s.tombstones[ts.Tag]; !already {
				s.tombstones[ts.Tag] = ts
			}
		}
	}

	// Adds.
	for i := range delta.Entries {
		add := &delta.Entries[i]

		if _, removed := s.tombstones[add.Tag]; removed {
			continue
		}

		existing, ok := s.entries[add.Key]
		if !ok {
			e := *add
			s.entries[add.Key] = &e
			s.clock.Merge(add.Timestamp)
			continue
		}

		// Conflict resolution:
		//  1. If the incoming write causally follows the existing one, it wins.
		//  2. If the existing write causally follows the incoming one, keep existing.
		//  3. If concurrent (neither dominates causally), deterministic tiebreak by tag.
		if add.Timestamp.HappensBefore(existing.Timestamp) {
			continue // existing is causally newer
		}
		if existing.Timestamp.HappensBefore(add.Timestamp) {
			e := *add
			s.entries[add.Key] = &e
			s.clock.Merge(add.Timestamp)
			continue
		}
		// Concurrent — deterministic tiebreak.
		if add.Tag.Dominates(existing.Tag) {
			e := *add
			s.entries[add.Key] = &e
			s.clock.Merge(add.Timestamp)
		}
	}
}

// --- Tombstone compaction ---

// Compact removes tombstones older than TombstoneTTL.
func (s *ORSet) Compact() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := s.now().Add(-s.TombstoneTTL)
	removed := 0
	for tag, ts := range s.tombstones {
		if ts.RemovedAt.Before(cutoff) {
			delete(s.tombstones, tag)
			removed++
		}
	}
	return removed
}

// TombstoneCount returns the number of active tombstones.
func (s *ORSet) TombstoneCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.tombstones)
}

// Size returns the number of live entries.
func (s *ORSet) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}
