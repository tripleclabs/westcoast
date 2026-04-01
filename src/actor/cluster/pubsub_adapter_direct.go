package cluster

import (
	"context"
	"encoding/gob"
	"time"
)

func init() {
	gob.Register(pubsubBroadcastMsg{})
}

// pubsubBroadcastMsg is the wire format for a cross-node publication.
type pubsubBroadcastMsg struct {
	Topic         string
	Payload       []byte // codec-encoded publish payload
	PublisherNode NodeID
}

// pubsubBroadcastDecoded carries the decoded payload for local delivery.
type pubsubBroadcastDecoded struct {
	Topic         string
	Payload       any
	PublisherNode NodeID
}

const pubsubEnvelopeType = "__pubsub_broadcast"

// DirectPubSubAdapter broadcasts publications to every other node in the
// cluster via direct sends. Simple and correct for small clusters.
// For larger clusters, use GossipPubSubAdapter.
type DirectPubSubAdapter struct {
	cluster *Cluster
	codec   Codec
	handler RemotePublishHandler
}

// NewDirectPubSubAdapter creates a new adapter that broadcasts via direct sends to all peers.
func NewDirectPubSubAdapter(cluster *Cluster, codec Codec) *DirectPubSubAdapter {
	return &DirectPubSubAdapter{
		cluster: cluster,
		codec:   codec,
	}
}

// Broadcast sends a publication to every other node in the cluster via direct sends.
func (a *DirectPubSubAdapter) Broadcast(ctx context.Context, topic string, payload any, publisherNode NodeID) error {
	payloadBytes, err := a.codec.Encode(payload)
	if err != nil {
		return err
	}
	msg := pubsubBroadcastMsg{
		Topic:         topic,
		Payload:       payloadBytes,
		PublisherNode: publisherNode,
	}
	encoded, err := a.codec.Encode(msg)
	if err != nil {
		return err
	}

	members := a.cluster.Members()
	for _, m := range members {
		env := Envelope{
			SenderNode:     a.cluster.LocalNodeID(),
			TargetNode:     m.ID,
			TypeName:       pubsubEnvelopeType,
			Payload:        encoded,
			SentAtUnixNano: time.Now().UnixNano(),
		}
		// Fire-and-forget — partial delivery is acceptable for pubsub.
		a.cluster.SendRemote(ctx, m.ID, env)
	}
	return nil
}

// SetHandler registers the callback for incoming remote publications.
func (a *DirectPubSubAdapter) SetHandler(handler RemotePublishHandler) {
	a.handler = handler
}

// HandleInbound processes an incoming pubsub broadcast envelope.
func (a *DirectPubSubAdapter) HandleInbound(env Envelope) {
	if a.handler == nil {
		return
	}
	var decoded any
	if err := a.codec.Decode(env.Payload, &decoded); err != nil {
		return
	}
	msg, ok := decoded.(pubsubBroadcastMsg)
	if !ok {
		return
	}
	// Decode the inner payload from bytes back to a Go value.
	var payload any
	if err := a.codec.Decode(msg.Payload, &payload); err != nil {
		return
	}
	a.handler(msg.Topic, payload, msg.PublisherNode)
}

// Start is a no-op for the direct adapter since it uses on-demand sends.
func (a *DirectPubSubAdapter) Start(ctx context.Context) error { return nil }

// Stop is a no-op for the direct adapter.
func (a *DirectPubSubAdapter) Stop() error { return nil }

// RegisterPubSubHandler registers a direct pubsub adapter with an
// InboundDispatcher so that pubsub broadcast envelopes are routed automatically.
func RegisterPubSubHandler(d *InboundDispatcher, adapter *DirectPubSubAdapter) {
	d.RegisterHandler(pubsubEnvelopeType, func(from NodeID, env Envelope) {
		adapter.HandleInbound(env)
	})
}
