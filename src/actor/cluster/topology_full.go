package cluster

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

func (FullMeshTopology) Responsible(key string, members []NodeMeta, replication int) []NodeID {
	if replication <= 0 {
		replication = 1
	}
	out := make([]NodeID, 0, replication)
	for i, m := range members {
		if i >= replication {
			break
		}
		out = append(out, m.ID)
	}
	return out
}
