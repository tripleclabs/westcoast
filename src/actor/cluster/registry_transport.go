package cluster

import (
	"context"
	"sync"
	"time"

	crdt "github.com/tripleclabs/crdt-go"
)

const registryMsgType = "__registry"

// ClusterCRDTTransport implements crdt.Transport by sending TransportMessages
// over the cluster's SendRemote as envelopes. Messages are opaque — the
// CRDT library handles serialization via Marshal/UnmarshalTransportMessage.
type ClusterCRDTTransport struct {
	cluster   *Cluster
	localID   NodeID
	replicaID crdt.ReplicaID

	mu      sync.Mutex
	recvFn  func(crdt.TransportMessage)
}

// NewClusterCRDTTransport creates a CRDT transport backed by the cluster.
func NewClusterCRDTTransport(cluster *Cluster) *ClusterCRDTTransport {
	localID := cluster.LocalNodeID()
	return &ClusterCRDTTransport{
		cluster:   cluster,
		localID:   localID,
		replicaID: nodeIDToReplicaID(localID),
	}
}

// RegisterHandler wires the transport to the inbound dispatcher so it
// receives messages from remote peers.
func (t *ClusterCRDTTransport) RegisterHandler(d *InboundDispatcher) {
	d.RegisterHandler(registryMsgType, t.handleInbound)
}

// Send implements crdt.Transport.
func (t *ClusterCRDTTransport) Send(ctx context.Context, peer crdt.ReplicaID, msg crdt.TransportMessage) (<-chan struct{}, error) {
	targetNode := t.replicaIDToNodeID(peer)
	if targetNode == "" {
		return nil, nil
	}

	env := Envelope{
		SenderNode:     t.localID,
		TargetNode:     targetNode,
		TypeName:       registryMsgType,
		Payload:        msg.Marshal(),
		SentAtUnixNano: time.Now().UnixNano(),
	}
	if err := t.cluster.SendRemote(ctx, targetNode, env); err != nil {
		return nil, err
	}
	return nil, nil
}

// OnReceive implements crdt.Transport.
func (t *ClusterCRDTTransport) OnReceive(fn func(crdt.TransportMessage)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.recvFn = fn
}

func (t *ClusterCRDTTransport) handleInbound(from NodeID, env Envelope) {
	t.mu.Lock()
	fn := t.recvFn
	t.mu.Unlock()

	if fn == nil {
		return
	}

	fromReplica := nodeIDToReplicaID(from)
	msg, ok := crdt.UnmarshalTransportMessage(fromReplica, env.Payload)
	if !ok {
		return
	}
	fn(msg)
}

func (t *ClusterCRDTTransport) replicaIDToNodeID(target crdt.ReplicaID) NodeID {
	for _, m := range t.cluster.Members() {
		if nodeIDToReplicaID(m.ID) == target {
			return m.ID
		}
	}
	return ""
}

var _ crdt.Transport = (*ClusterCRDTTransport)(nil)

// ClusterTopology implements crdt.TopologyProvider using the cluster's
// current membership.
type ClusterTopology struct {
	cluster *Cluster
}

// NewClusterTopology creates a topology provider backed by cluster membership.
func NewClusterTopology(cluster *Cluster) *ClusterTopology {
	return &ClusterTopology{cluster: cluster}
}

// Peers returns the replica IDs of all current cluster members (excluding self).
func (ct *ClusterTopology) Peers() []crdt.ReplicaID {
	members := ct.cluster.Members()
	peers := make([]crdt.ReplicaID, len(members))
	for i, m := range members {
		peers[i] = nodeIDToReplicaID(m.ID)
	}
	return peers
}

var _ crdt.TopologyProvider = (*ClusterTopology)(nil)
