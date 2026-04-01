package cluster

import (
	"hash/fnv"
	"math"
	"sort"
)

// RingTopology arranges nodes on a consistent hash ring. Each node
// connects to its N nearest neighbors (fanout) plus a set of finger
// table entries at exponentially increasing distances, giving O(log n)
// routing with bounded connection count per node.
//
// For clusters smaller than 2*fanout, it degrades to full mesh.
type RingTopology struct {
	// Fanout is the number of direct neighbors in each direction on the ring.
	// Total neighbor connections = 2 * Fanout.
	Fanout int
	// VNodes is the number of virtual nodes per physical node for
	// more even distribution on the ring. Defaults to 1 if zero.
	VNodes int
}

// NewRingTopology creates a RingTopology with the given fanout and virtual node count, clamping both to at least 1.
func NewRingTopology(fanout, vnodes int) *RingTopology {
	if fanout < 1 {
		fanout = 1
	}
	if vnodes < 1 {
		vnodes = 1
	}
	return &RingTopology{Fanout: fanout, VNodes: vnodes}
}

// ShouldConnect returns the nodes this node should maintain direct
// connections to: immediate ring neighbors + finger table targets.
func (r *RingTopology) ShouldConnect(self NodeID, members []NodeMeta) []NodeID {
	ring := r.buildRing(self, members)
	if ring == nil {
		return nil
	}

	// For small clusters, connect to everyone.
	if len(ring.nodes) <= 2*r.Fanout+1 {
		out := make([]NodeID, 0, len(ring.nodes)-1)
		for _, n := range ring.nodes {
			if n != self {
				out = append(out, n)
			}
		}
		return dedupNodes(out)
	}

	targets := make(map[NodeID]bool)

	// Add immediate neighbors (fanout in each direction).
	selfIdx := ring.indexOf(self)
	if selfIdx < 0 {
		return nil
	}
	n := len(ring.nodes)
	for i := 1; i <= r.Fanout; i++ {
		targets[ring.nodes[(selfIdx+i)%n]] = true
		targets[ring.nodes[(selfIdx-i+n)%n]] = true
	}

	// Add finger table entries.
	fingers := r.buildFingers(ring, selfIdx)
	for _, f := range fingers {
		targets[f] = true
	}

	delete(targets, self)
	out := make([]NodeID, 0, len(targets))
	for id := range targets {
		out = append(out, id)
	}
	return out
}

// Route returns the next hop to reach target from self.
func (r *RingTopology) Route(self, target NodeID, members []NodeMeta) (NodeID, bool) {
	ring := r.buildRing(self, members)
	if ring == nil {
		return "", false
	}

	// If target is not in the ring, unreachable.
	targetIdx := ring.indexOf(target)
	if targetIdx < 0 {
		return "", false
	}

	selfIdx := ring.indexOf(self)
	if selfIdx < 0 {
		return "", false
	}

	// Direct neighbor check — can route directly.
	n := len(ring.nodes)
	dist := ringDistance(selfIdx, targetIdx, n)
	if dist <= r.Fanout {
		return target, true
	}

	// Check finger table for the best next hop.
	fingers := r.buildFingers(ring, selfIdx)
	bestHop := NodeID("")
	bestDist := n // worst case

	for _, f := range fingers {
		fIdx := ring.indexOf(f)
		if fIdx < 0 {
			continue
		}
		fToTarget := ringDistance(fIdx, targetIdx, n)
		if fToTarget < bestDist {
			bestDist = fToTarget
			bestHop = f
		}
	}

	// Also check immediate neighbors as candidates.
	for i := 1; i <= r.Fanout; i++ {
		neighbor := ring.nodes[(selfIdx+i)%n]
		nIdx := ring.indexOf(neighbor)
		nToTarget := ringDistance(nIdx, targetIdx, n)
		if nToTarget < bestDist {
			bestDist = nToTarget
			bestHop = neighbor
		}
		neighbor = ring.nodes[(selfIdx-i+n)%n]
		nIdx = ring.indexOf(neighbor)
		nToTarget = ringDistance(nIdx, targetIdx, n)
		if nToTarget < bestDist {
			bestDist = nToTarget
			bestHop = neighbor
		}
	}

	if bestHop == "" {
		return "", false
	}
	return bestHop, true
}

// Responsible returns the nodes responsible for a given key using
// consistent hashing. Returns up to `replication` nodes, starting
// from the node that owns the key's hash position and proceeding
// clockwise on the ring.
func (r *RingTopology) Responsible(key string, members []NodeMeta, replication int) []NodeID {
	if replication <= 0 {
		replication = 1
	}
	if len(members) == 0 {
		return nil
	}

	ring := r.buildRingFromMembers(members)
	keyHash := hashString(key)

	// Find the first vnode position >= keyHash.
	idx := sort.Search(len(ring.positions), func(i int) bool {
		return ring.positions[i].hash >= keyHash
	})
	if idx >= len(ring.positions) {
		idx = 0 // wrap around
	}

	// Walk clockwise, collecting unique physical nodes.
	seen := make(map[NodeID]bool)
	var out []NodeID
	for i := 0; i < len(ring.positions) && len(out) < replication; i++ {
		pos := ring.positions[(idx+i)%len(ring.positions)]
		if !seen[pos.node] {
			seen[pos.node] = true
			out = append(out, pos.node)
		}
	}
	return out
}

// --- internal ring representation ---

type ring struct {
	nodes     []NodeID       // unique physical nodes in ring order
	positions []vnodePos     // all vnode positions sorted by hash
	nodeIndex map[NodeID]int // node -> index in nodes slice
}

type vnodePos struct {
	hash uint64
	node NodeID
}

func (rg *ring) indexOf(id NodeID) int {
	if idx, ok := rg.nodeIndex[id]; ok {
		return idx
	}
	return -1
}

// buildRing creates a ring of unique physical nodes ordered by their
// primary hash position. Includes self and all members.
func (r *RingTopology) buildRing(self NodeID, members []NodeMeta) *ring {
	allNodes := make(map[NodeID]bool)
	allNodes[self] = true
	for _, m := range members {
		allNodes[m.ID] = true
	}

	if len(allNodes) < 2 {
		return nil
	}

	return r.buildRingFromNodeSet(allNodes)
}

func (r *RingTopology) buildRingFromMembers(members []NodeMeta) *ring {
	allNodes := make(map[NodeID]bool)
	for _, m := range members {
		allNodes[m.ID] = true
	}
	return r.buildRingFromNodeSet(allNodes)
}

func (r *RingTopology) buildRingFromNodeSet(allNodes map[NodeID]bool) *ring {
	// Build vnode positions.
	var positions []vnodePos
	for id := range allNodes {
		for v := 0; v < r.VNodes; v++ {
			h := hashVNode(id, v)
			positions = append(positions, vnodePos{hash: h, node: id})
		}
	}
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].hash < positions[j].hash
	})

	// Build unique node list ordered by primary (vnode 0) hash.
	type nodeHash struct {
		id   NodeID
		hash uint64
	}
	var nh []nodeHash
	for id := range allNodes {
		nh = append(nh, nodeHash{id: id, hash: hashVNode(id, 0)})
	}
	sort.Slice(nh, func(i, j int) bool {
		return nh[i].hash < nh[j].hash
	})

	nodes := make([]NodeID, len(nh))
	nodeIndex := make(map[NodeID]int, len(nh))
	for i, n := range nh {
		nodes[i] = n.id
		nodeIndex[n.id] = i
	}

	return &ring{
		nodes:     nodes,
		positions: positions,
		nodeIndex: nodeIndex,
	}
}

// buildFingers returns finger table entries: nodes at exponentially
// increasing distances around the ring. Gives O(log n) routing.
func (r *RingTopology) buildFingers(rg *ring, selfIdx int) []NodeID {
	n := len(rg.nodes)
	if n <= 2 {
		return nil
	}

	// Number of fingers: log2(n), at least 1.
	numFingers := int(math.Log2(float64(n)))
	if numFingers < 1 {
		numFingers = 1
	}

	fingers := make([]NodeID, 0, numFingers)
	seen := make(map[NodeID]bool)

	for i := 0; i < numFingers; i++ {
		// Jump 2^i positions clockwise on the ring.
		jump := 1 << uint(i)
		idx := (selfIdx + jump) % n
		node := rg.nodes[idx]
		if node != rg.nodes[selfIdx] && !seen[node] {
			seen[node] = true
			fingers = append(fingers, node)
		}
	}
	return fingers
}

// --- hashing ---

func hashString(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func hashVNode(id NodeID, vnode int) uint64 {
	// Hash the combined key using two rounds of FNV to ensure good
	// distribution even for short, similar node IDs.
	h := fnv.New64a()
	h.Write([]byte(id))
	first := h.Sum64()
	// Mix the vnode index into the first hash to spread positions.
	// This is effectively hash(hash(nodeID) ^ vnodeIndex).
	h.Reset()
	mixed := first ^ uint64(vnode)*0x9e3779b97f4a7c15 // golden ratio constant
	h.Write([]byte{
		byte(mixed >> 56), byte(mixed >> 48), byte(mixed >> 40), byte(mixed >> 32),
		byte(mixed >> 24), byte(mixed >> 16), byte(mixed >> 8), byte(mixed),
	})
	return h.Sum64()
}

// ringDistance returns the clockwise distance from a to b on a ring of size n.
func ringDistance(a, b, n int) int {
	d := b - a
	if d < 0 {
		d += n
	}
	return d
}

func dedupNodes(nodes []NodeID) []NodeID {
	seen := make(map[NodeID]bool, len(nodes))
	out := make([]NodeID, 0, len(nodes))
	for _, n := range nodes {
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}
