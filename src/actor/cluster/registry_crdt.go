package cluster

import (
	"encoding/gob"

	"github.com/tripleclabs/westcoast/src/actor"
	"github.com/tripleclabs/westcoast/src/crdt"
)

func init() {
	gob.Register(actor.PID{})
	gob.Register(crdt.Entry{})
	gob.Register(crdt.StateDelta{})
	gob.Register(crdt.Digest{})
}

// CRDTRegistry is a distributed name registry backed by a crdt.ORSet.
// It maps names (strings) to actor PIDs, with eventual consistency
// across the cluster via digest-based anti-entropy gossip.
//
// Node ownership is derived from the PID's Namespace field — the CRDT
// layer doesn't need to know about nodes.
type CRDTRegistry struct {
	set *crdt.ORSet
}

func NewCRDTRegistry(nodeID NodeID) *CRDTRegistry {
	return &CRDTRegistry{
		set: crdt.NewORSet(crdt.ORSetConfig{NodeID: string(nodeID)}),
	}
}

func (r *CRDTRegistry) Register(name string, pid actor.PID) error {
	// Check if already registered to the same PID (idempotent).
	if e := r.set.Lookup(name); e != nil {
		if e.Value.(actor.PID).Key() == pid.Key() {
			return nil
		}
	}
	return r.set.Add(name, pid)
}

func (r *CRDTRegistry) Lookup(name string) (actor.PID, bool) {
	e := r.set.Lookup(name)
	if e == nil {
		return actor.PID{}, false
	}
	return e.Value.(actor.PID), true
}

func (r *CRDTRegistry) Unregister(name string) (actor.PID, bool) {
	e := r.set.Remove(name)
	if e == nil {
		return actor.PID{}, false
	}
	return e.Value.(actor.PID), true
}

// UnregisterByNode removes all entries where the PID lives on the given node.
// Node ownership is derived from PID.Namespace.
func (r *CRDTRegistry) UnregisterByNode(node NodeID) []string {
	return r.set.RemoveIf(func(e crdt.Entry) bool {
		return e.Value.(actor.PID).Namespace == string(node)
	})
}

func (r *CRDTRegistry) OnMembershipChange(event MemberEvent) {
	if event.Type == MemberFailed || event.Type == MemberLeave {
		r.UnregisterByNode(event.Member.ID)
	}
}

// NamesByNode returns all names where the PID lives on the given node.
func (r *CRDTRegistry) NamesByNode(node NodeID) []string {
	entries := r.set.Filter(func(e crdt.Entry) bool {
		return e.Value.(actor.PID).Namespace == string(node)
	})
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Key
	}
	return out
}

// AllEntries returns all registered name→PID bindings.
func (r *CRDTRegistry) AllEntries() map[string]actor.PID {
	entries := r.set.Entries()
	out := make(map[string]actor.PID, len(entries))
	for _, e := range entries {
		out[e.Key] = e.Value.(actor.PID)
	}
	return out
}

// --- Gossip integration ---

func (r *CRDTRegistry) Digest() crdt.Digest        { return r.set.Digest() }
func (r *CRDTRegistry) DeltaFor(d crdt.Digest) crdt.StateDelta { return r.set.DeltaFor(d) }
func (r *CRDTRegistry) MergeDelta(d crdt.StateDelta)           { r.set.MergeDelta(d) }
func (r *CRDTRegistry) Compact() int                           { return r.set.Compact() }
