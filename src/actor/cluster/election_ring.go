package cluster

import (
	"hash/fnv"
	"sort"
	"sync"
)

// RingElection implements LeaderElection using deterministic hash-ring
// position. The leader for a scope is the node whose hash of
// (scope + nodeID) is the lowest — all nodes compute the same answer
// from the same membership list, so no voting protocol is needed.
//
// When membership changes (join/leave/fail), leadership is recomputed.
// If the leader changes, the term increments and watchers are notified.
type RingElection struct {
	localID NodeID

	mu      sync.RWMutex
	members map[NodeID]bool           // live members including self
	scopes  map[string]*electionScope // scope → state
}

type electionScope struct {
	leader   NodeID
	term     uint64
	watchers []chan LeaderEvent
}

// NewRingElection creates a new RingElection for the given local node.
func NewRingElection(localID NodeID) *RingElection {
	return &RingElection{
		localID: localID,
		members: map[NodeID]bool{localID: true},
		scopes:  make(map[string]*electionScope),
	}
}

// Leader returns the current leader for the scope, or ("", false) if none.
func (e *RingElection) Leader(scope string) (NodeID, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	s := e.ensureScopeLocked(scope)
	return s.leader, s.leader != ""
}

// IsLeader returns true if this node is the leader for the scope.
func (e *RingElection) IsLeader(scope string) bool {
	leader, ok := e.Leader(scope)
	return ok && leader == e.localID
}

// Term returns the current election term for the scope.
func (e *RingElection) Term(scope string) uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.scopes[scope]
	if !ok {
		return 0
	}
	return s.term
}

// Watch returns a channel that emits leadership changes for the scope.
func (e *RingElection) Watch(scope string) <-chan LeaderEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	s := e.ensureScopeLocked(scope)
	ch := make(chan LeaderEvent, 4)
	s.watchers = append(s.watchers, ch)

	// Immediately emit the current state if there's a leader.
	if s.leader != "" {
		select {
		case ch <- LeaderEvent{Scope: scope, Leader: s.leader, Term: s.term}:
		default:
		}
	}

	return ch
}

// OnMembershipChange updates the member set and recomputes leaders for all active scopes.
func (e *RingElection) OnMembershipChange(event MemberEvent) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch event.Type {
	case MemberJoin, MemberUpdated:
		e.members[event.Member.ID] = true
	case MemberLeave, MemberFailed:
		delete(e.members, event.Member.ID)
	}

	// Recompute all active scopes.
	for scope, s := range e.scopes {
		newLeader := e.computeLeaderLocked(scope)
		if newLeader != s.leader {
			prev := s.leader
			s.leader = newLeader
			s.term++
			e.notifyWatchersLocked(s, LeaderEvent{
				Scope:      scope,
				Leader:     newLeader,
				PrevLeader: prev,
				Term:       s.term,
			})
		}
	}
}

// SetMembers replaces the full membership set. Used during initialization.
func (e *RingElection) SetMembers(members []NodeMeta) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.members = map[NodeID]bool{e.localID: true}
	for _, m := range members {
		e.members[m.ID] = true
	}
	// Recompute all scopes.
	for scope, s := range e.scopes {
		newLeader := e.computeLeaderLocked(scope)
		if newLeader != s.leader {
			prev := s.leader
			s.leader = newLeader
			s.term++
			e.notifyWatchersLocked(s, LeaderEvent{
				Scope:      scope,
				Leader:     newLeader,
				PrevLeader: prev,
				Term:       s.term,
			})
		}
	}
}

func (e *RingElection) memberCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.members)
}

// leaderExcluding computes who would be leader for a scope if the given
// node were removed from the election. Returns "" if no other members exist.
func (e *RingElection) leaderExcluding(scope string, exclude NodeID) NodeID {
	e.mu.RLock()
	defer e.mu.RUnlock()
	others := make(map[NodeID]bool, len(e.members)-1)
	for id := range e.members {
		if id != exclude {
			others[id] = true
		}
	}
	return computeLeader(scope, others)
}

func (e *RingElection) ensureScopeLocked(scope string) *electionScope {
	s, ok := e.scopes[scope]
	if !ok {
		leader := e.computeLeaderLocked(scope)
		s = &electionScope{leader: leader, term: 1}
		e.scopes[scope] = s
	}
	return s
}

// computeLeaderLocked determines the leader for a scope by hashing
// (scope + nodeID) for each member and picking the lowest hash.
func (e *RingElection) computeLeaderLocked(scope string) NodeID {
	return computeLeader(scope, e.members)
}

// LeaderAmong computes the leader for a scope considering only nodes
// that pass the matcher. Returns ("", false) if no candidates match.
// This is a read-only query — it does not affect stored scope state.
func (e *RingElection) LeaderAmong(scope string, matcher NodeMatcher, allMembers []NodeMeta) (NodeID, bool) {
	candidates := FilterNodes(allMembers, matcher)
	if len(candidates) == 0 {
		return "", false
	}
	filtered := make(map[NodeID]bool, len(candidates))
	for _, c := range candidates {
		filtered[c.ID] = true
	}
	return computeLeader(scope, filtered), true
}

// computeLeader picks the node with the lowest hash(scope + nodeID).
func computeLeader(scope string, members map[NodeID]bool) NodeID {
	if len(members) == 0 {
		return ""
	}
	type candidate struct {
		id   NodeID
		hash uint64
	}
	candidates := make([]candidate, 0, len(members))
	for id := range members {
		h := fnv.New64a()
		h.Write([]byte(scope))
		h.Write([]byte{0})
		h.Write([]byte(id))
		candidates = append(candidates, candidate{id: id, hash: h.Sum64()})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hash < candidates[j].hash
	})
	return candidates[0].id
}

func (e *RingElection) notifyWatchersLocked(s *electionScope, ev LeaderEvent) {
	for _, ch := range s.watchers {
		select {
		case ch <- ev:
		default:
			// Drop if watcher is slow — they'll get the next event.
		}
	}
}
