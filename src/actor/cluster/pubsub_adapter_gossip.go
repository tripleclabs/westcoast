package cluster

import (
	"context"
	"encoding/gob"
	"fmt"
	"sync"
)

func init() {
	gob.Register(pubsubGossipPayload{})
}

const pubsubGossipProtocol = "pubsub_broadcast"

// pubsubGossipPayload wraps a publication for gossip transport.
type pubsubGossipPayload struct {
	Topic         string
	Payload       []byte // codec-encoded
	PublisherNode NodeID
}

const maxPendingPubSub = 4096

// GossipPubSubAdapter broadcasts publications via the gossip protocol.
// Publications are batched and sent on gossip ticks to `fanout` random
// peers. Suitable for larger clusters where direct fan-out is too expensive.
// For latency-sensitive use cases, prefer DirectPubSubAdapter.
type GossipPubSubAdapter struct {
	cluster *Cluster
	codec   Codec
	gossip  *GossipProtocol
	handler RemotePublishHandler

	mu      sync.Mutex
	pending []pubsubGossipPayload
}

// NewGossipPubSubAdapter creates a new adapter that broadcasts publications via gossip.
func NewGossipPubSubAdapter(cluster *Cluster, codec Codec, cfg GossipConfig) *GossipPubSubAdapter {
	a := &GossipPubSubAdapter{
		cluster: cluster,
		codec:   codec,
	}

	cfg.Protocol = pubsubGossipProtocol
	cfg.OnProduce = a.produce
	cfg.OnReceive = a.receive

	a.gossip = NewGossipProtocol(cluster, codec, cfg)
	return a
}

// Broadcast enqueues a publication for the next gossip round.
func (a *GossipPubSubAdapter) Broadcast(ctx context.Context, topic string, payload any, publisherNode NodeID) error {
	encoded, err := a.codec.Encode(payload)
	if err != nil {
		return err
	}
	a.mu.Lock()
	if len(a.pending) >= maxPendingPubSub {
		a.mu.Unlock()
		return fmt.Errorf("pubsub gossip buffer full (%d)", maxPendingPubSub)
	}
	a.pending = append(a.pending, pubsubGossipPayload{
		Topic:         topic,
		Payload:       encoded,
		PublisherNode: publisherNode,
	})
	a.mu.Unlock()
	return nil
}

// SetHandler registers the callback for incoming remote publications received via gossip.
func (a *GossipPubSubAdapter) SetHandler(handler RemotePublishHandler) {
	a.handler = handler
}

// Start begins the gossip protocol for pubsub broadcast.
func (a *GossipPubSubAdapter) Start(ctx context.Context) error {
	a.gossip.Start(ctx)
	return nil
}

// Stop shuts down the gossip protocol and releases resources.
func (a *GossipPubSubAdapter) Stop() error {
	a.gossip.Stop()
	return nil
}

// GossipProtocol returns the underlying gossip for inbound routing.
func (a *GossipPubSubAdapter) GossipProtocol() *GossipProtocol {
	return a.gossip
}

func (a *GossipPubSubAdapter) produce() []byte {
	a.mu.Lock()
	if len(a.pending) == 0 {
		a.mu.Unlock()
		return nil
	}
	batch := a.pending
	a.pending = nil
	a.mu.Unlock()

	encoded, err := a.codec.Encode(batch)
	if err != nil {
		return nil
	}
	return encoded
}

func (a *GossipPubSubAdapter) receive(from NodeID, data []byte) {
	if a.handler == nil {
		return
	}

	var decoded any
	if err := a.codec.Decode(data, &decoded); err != nil {
		return
	}

	batch, ok := decoded.([]pubsubGossipPayload)
	if !ok {
		return
	}

	for _, msg := range batch {
		var payload any
		if err := a.codec.Decode(msg.Payload, &payload); err != nil {
			continue
		}
		a.handler(msg.Topic, payload, msg.PublisherNode)
	}
}
