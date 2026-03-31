package cluster

import "time"

// NodeID uniquely identifies a node in the cluster.
// This value is used as the PID.Namespace for actors on this node.
type NodeID string

// NodeMeta describes a cluster member.
type NodeMeta struct {
	ID       NodeID
	Addr     string            // host:port for transport connections
	Tags     map[string]string // arbitrary metadata (region, zone, capabilities)
	JoinedAt time.Time
}
