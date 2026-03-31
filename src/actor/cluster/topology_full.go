package cluster

import (
	"hash/fnv"
	"sort"
)

// FullMeshTopology connects every node to every other node.
// Routing is always direct (single hop). This is the default
// topology and works well for small clusters (< ~20 nodes).
type FullMeshTopology struct{}

func (FullMeshTopology) ShouldConnect(self NodeID, members []NodeMeta) []NodeID {
	out := make([]NodeID, 0, len(members))
	for _, m := range members {
		if m.ID != self {
			out = append(out, m.ID)
		}
	}
	return out
}

func (FullMeshTopology) Route(self, target NodeID, members []NodeMeta) (NodeID, bool) {
	for _, m := range members {
		if m.ID == target {
			return target, true
		}
	}
	return "", false
}

// Responsible returns the nodes responsible for a key using consistent
// hashing. Key ownership is deterministic across all nodes.
func (FullMeshTopology) Responsible(key string, members []NodeMeta, replication int) []NodeID {
	if replication <= 0 {
		replication = 1
	}
	if len(members) == 0 {
		return nil
	}

	// Hash each node with the key for deterministic placement.
	type candidate struct {
		id   NodeID
		hash uint64
	}
	candidates := make([]candidate, len(members))
	for i, m := range members {
		h := fnv.New64a()
		h.Write([]byte(key))
		h.Write([]byte{0})
		h.Write([]byte(m.ID))
		candidates[i] = candidate{id: m.ID, hash: h.Sum64()}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].hash < candidates[j].hash
	})

	if replication > len(candidates) {
		replication = len(candidates)
	}
	out := make([]NodeID, replication)
	for i := 0; i < replication; i++ {
		out[i] = candidates[i].id
	}
	return out
}
