package cluster

import (
	"context"
	"encoding/gob"
	"time"

	crdt "github.com/tripleclabs/crdt-go"
)

func init() {
	gob.Register(pubsubRoutedMsg{})
}

// pubsubRoutedMsg is the wire format for a targeted publication.
type pubsubRoutedMsg struct {
	Topic         string
	Payload       []byte // codec-encoded
	PublisherNode NodeID
}

const pubsubRoutedType = "__pubsub_routed"

// Key prefixes for the two views in the DeltaMap.
const (
	routeNodePrefix  = "node:"  // node:<nodeID> → ORSet of topics
	routeTopicPrefix = "topic:" // topic:<topic> → ORSet of node IDs
)

// CRDTPubSubAdapter broadcasts publications only to nodes that have
// subscribers for the topic. Subscription routing is maintained via a
// crdt.DeltaMap with two key namespaces:
//
//   - "node:<nodeID>" → ORSet of topic patterns (for cascade cleanup)
//   - "topic:<pattern>" → ORSet of node IDs (for fast publish lookup)
//
// Each node is the sole writer for its own entries. Publish reads
// "topic:<pattern>" directly — no scanning. Node departure calls
// RemoveKey on "node:<nodeID>" for cascade cleanup, then removes
// the node from each topic set.
type CRDTPubSubAdapter struct {
	cluster *Cluster
	codec   Codec
	localID NodeID
	handler RemotePublishHandler

	routes *crdt.DeltaMap[crdt.ORSetKind[string]]
}

// NewCRDTPubSubAdapter creates a pubsub adapter with CRDT-replicated
// subscription routing.
func NewCRDTPubSubAdapter(cluster *Cluster, codec Codec, transport crdt.Transport, topology crdt.TopologyProvider) *CRDTPubSubAdapter {
	localID := cluster.LocalNodeID()
	replicaID := nodeIDToReplicaID(localID)

	return &CRDTPubSubAdapter{
		cluster: cluster,
		codec:   codec,
		localID: localID,
		routes: crdt.NewDeltaMap(replicaID,
			crdt.ORSetKind[string]{Codec: crdt.StringCodec{}},
			crdt.WithTransport(transport),
			crdt.WithTopology(topology),
		),
	}
}

// NotifySubscribe registers that this node has a local subscriber for
// the given topic. Writes both views: adds the topic to the node's set
// and adds the node to the topic's set.
func (a *CRDTPubSubAdapter) NotifySubscribe(ctx context.Context, topic string) {
	nodeKey := routeNodePrefix + string(a.localID)
	topicKey := routeTopicPrefix + topic

	a.routes.Mutate(ctx, nodeKey, crdt.AddSetMember[string]{Value: topic})
	a.routes.Mutate(ctx, topicKey, crdt.AddSetMember[string]{Value: string(a.localID)})
}

// NotifyUnsubscribe removes this node's subscription for the given topic.
// Writes both views.
func (a *CRDTPubSubAdapter) NotifyUnsubscribe(ctx context.Context, topic string) {
	nodeKey := routeNodePrefix + string(a.localID)
	topicKey := routeTopicPrefix + topic

	a.routes.Mutate(ctx, nodeKey, crdt.RemoveSetMember[string]{Value: topic})
	a.routes.Mutate(ctx, topicKey, crdt.RemoveSetMember[string]{Value: string(a.localID)})
}

// Broadcast sends a publication only to nodes that have subscribers
// for the topic. Reads the topic's node set directly — no scanning.
func (a *CRDTPubSubAdapter) Broadcast(ctx context.Context, topic string, payload any, publisherNode NodeID) error {
	payloadBytes, err := a.codec.Encode(payload)
	if err != nil {
		return err
	}

	msg := pubsubRoutedMsg{
		Topic:         topic,
		Payload:       payloadBytes,
		PublisherNode: publisherNode,
	}
	encoded, err := a.codec.Encode(msg)
	if err != nil {
		return err
	}

	topicKey := routeTopicPrefix + topic
	if !a.routes.HasKey(topicKey) {
		return nil // no subscribers anywhere
	}

	nodes := a.routes.Query(topicKey, crdt.SetElements[string]{}).([]string)

	for _, nodeStr := range nodes {
		nodeID := NodeID(nodeStr)
		if nodeID == a.localID {
			continue
		}
		env := Envelope{
			SenderNode:     a.localID,
			TargetNode:     nodeID,
			TypeName:       pubsubRoutedType,
			Payload:        encoded,
			SentAtUnixNano: time.Now().UnixNano(),
		}
		a.cluster.SendRemote(ctx, nodeID, env)
	}
	return nil
}

// SetHandler registers the callback for incoming remote publications.
func (a *CRDTPubSubAdapter) SetHandler(handler RemotePublishHandler) {
	a.handler = handler
}

// RegisterHandler wires the adapter to the inbound dispatcher.
func (a *CRDTPubSubAdapter) RegisterHandler(d *InboundDispatcher) {
	d.RegisterHandler(pubsubRoutedType, a.handleInbound)
}

// Start is a no-op — the CRDT handles its own anti-entropy.
func (a *CRDTPubSubAdapter) Start(ctx context.Context) error { return nil }

// Stop closes the CRDT routing map.
func (a *CRDTPubSubAdapter) Stop() error {
	a.routes.Close()
	return nil
}

// OnMembershipChange cleans up routing entries for departed nodes.
// Removes the node's entry (cascade prunes its topic list) and removes
// the node from each topic's subscriber set.
func (a *CRDTPubSubAdapter) OnMembershipChange(event MemberEvent) {
	if event.Type != MemberFailed && event.Type != MemberLeave {
		return
	}

	nodeID := string(event.Member.ID)
	nodeKey := routeNodePrefix + nodeID
	ctx := context.Background()

	// Get the node's topics before removing.
	if a.routes.HasKey(nodeKey) {
		topics := a.routes.Query(nodeKey, crdt.SetElements[string]{}).([]string)

		// Remove this node from each topic's subscriber set.
		for _, topic := range topics {
			topicKey := routeTopicPrefix + topic
			a.routes.Mutate(ctx, topicKey, crdt.RemoveSetMember[string]{Value: nodeID})
		}

		// Cascade remove the node entry.
		a.routes.RemoveKey(ctx, nodeKey)
	}
}

func (a *CRDTPubSubAdapter) handleInbound(from NodeID, env Envelope) {
	if a.handler == nil {
		return
	}
	var raw any
	if err := a.codec.Decode(env.Payload, &raw); err != nil {
		return
	}
	msg, ok := raw.(pubsubRoutedMsg)
	if !ok {
		return
	}
	var payload any
	if err := a.codec.Decode(msg.Payload, &payload); err != nil {
		return
	}
	a.handler(msg.Topic, payload, msg.PublisherNode)
}

var _ PubSubAdapter = (*CRDTPubSubAdapter)(nil)
