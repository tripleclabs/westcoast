package cluster

import (
	"fmt"
	"sync"

	"github.com/tripleclabs/westcoast/src/actor"
)

// PartitionedRegistry shards names across cluster nodes using the topology's
// consistent hash ring. Each name has a deterministic "home node" — the node
// responsible for that name's hash position.
//
// Properties:
//   - Register/Unregister: only succeed on the home node. Remote callers get
//     ErrNotHomeNode and must forward to the correct node.
//   - Lookup: local lookups check the local shard. For names homed elsewhere,
//     the caller must forward to the home node (or use a cluster-wide lookup).
//   - Rebalance: when membership changes, names are redistributed. Names that
//     moved off this node are removed from the local shard.
//
// This gives strong consistency for name ownership (no split-brain for a given
// name) at the cost of requiring forwarding for non-local names.
type PartitionedRegistry struct {
	nodeID   NodeID
	topology Topology

	mu      sync.RWMutex
	entries map[string]actor.PID // name → PID (only names homed here)
	members []NodeMeta           // current cluster membership
}

var ErrNotHomeNode = fmt.Errorf("partitioned_registry_not_home_node")

func NewPartitionedRegistry(nodeID NodeID, topology Topology) *PartitionedRegistry {
	return &PartitionedRegistry{
		nodeID:   nodeID,
		topology: topology,
		entries:  make(map[string]actor.PID),
	}
}

func (r *PartitionedRegistry) Register(name string, pid actor.PID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	home := r.homeNode(name)
	if home != r.nodeID {
		return fmt.Errorf("%w: %s is homed on %s", ErrNotHomeNode, name, home)
	}

	if existing, ok := r.entries[name]; ok {
		if existing.Key() == pid.Key() {
			return nil // idempotent
		}
		return fmt.Errorf("name %q already registered to %s", name, existing.Key())
	}

	r.entries[name] = pid
	return nil
}

func (r *PartitionedRegistry) Lookup(name string) (actor.PID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pid, ok := r.entries[name]
	return pid, ok
}

// HomeNode returns the node responsible for the given name.
func (r *PartitionedRegistry) HomeNode(name string) NodeID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.homeNode(name)
}

func (r *PartitionedRegistry) Unregister(name string) (actor.PID, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	home := r.homeNode(name)
	if home != r.nodeID {
		return actor.PID{}, false
	}

	pid, ok := r.entries[name]
	if !ok {
		return actor.PID{}, false
	}
	delete(r.entries, name)
	return pid, true
}

func (r *PartitionedRegistry) UnregisterByNode(node NodeID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var removed []string
	for name, pid := range r.entries {
		if pid.Namespace == string(node) {
			delete(r.entries, name)
			removed = append(removed, name)
		}
	}
	return removed
}

func (r *PartitionedRegistry) OnMembershipChange(event MemberEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch event.Type {
	case MemberJoin, MemberUpdated:
		r.addMemberLocked(event.Member)
	case MemberLeave, MemberFailed:
		r.removeMemberLocked(event.Member.ID)
	}

	// Rebalance: remove entries that are no longer homed on this node.
	r.rebalanceLocked()
}

// SetMembers replaces the full membership list. Used during initialization.
func (r *PartitionedRegistry) SetMembers(members []NodeMeta) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.members = make([]NodeMeta, len(members))
	copy(r.members, members)
	r.rebalanceLocked()
}

// AllEntries returns all locally-homed entries.
func (r *PartitionedRegistry) AllEntries() map[string]actor.PID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make(map[string]actor.PID, len(r.entries))
	for name, pid := range r.entries {
		out[name] = pid
	}
	return out
}

// --- internal ---

func (r *PartitionedRegistry) homeNode(name string) NodeID {
	if len(r.members) == 0 {
		return r.nodeID // no cluster info, assume local
	}
	responsible := r.topology.Responsible(name, r.members, 1)
	if len(responsible) == 0 {
		return r.nodeID
	}
	return responsible[0]
}

func (r *PartitionedRegistry) rebalanceLocked() {
	for name := range r.entries {
		if r.homeNode(name) != r.nodeID {
			delete(r.entries, name)
		}
	}
}

func (r *PartitionedRegistry) addMemberLocked(meta NodeMeta) {
	for i, m := range r.members {
		if m.ID == meta.ID {
			r.members[i] = meta
			return
		}
	}
	r.members = append(r.members, meta)
}

func (r *PartitionedRegistry) removeMemberLocked(id NodeID) {
	for i, m := range r.members {
		if m.ID == id {
			r.members = append(r.members[:i], r.members[i+1:]...)
			return
		}
	}
}
