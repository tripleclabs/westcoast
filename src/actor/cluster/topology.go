package cluster

// Topology determines which nodes should maintain direct connections
// and how messages are routed through the cluster. Implementations
// range from full mesh (all-to-all) to ring-based (O(log n) hops).
type Topology interface {
	// ShouldConnect returns the set of node IDs that self should maintain
	// direct connections to, given the current membership.
	ShouldConnect(self NodeID, members []NodeMeta) []NodeID

	// Route returns the next hop for reaching target from self.
	// If self has a direct connection to target, returns target.
	// Otherwise returns an intermediate node that is closer to target
	// on the topology. Returns ("", false) if target is unreachable.
	Route(self, target NodeID, members []NodeMeta) (nextHop NodeID, ok bool)

	// Responsible returns the node(s) responsible for a given key,
	// used for partitioned data placement. The replication parameter
	// controls how many nodes share responsibility (for redundancy).
	Responsible(key string, members []NodeMeta, replication int) []NodeID
}
