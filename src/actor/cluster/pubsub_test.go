package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// pubsubCollector is an actor handler that collects BrokerPublishedMessages.
type pubsubCollector struct {
	mu       sync.Mutex
	messages []actor.BrokerPublishedMessage
	ch       chan actor.BrokerPublishedMessage
}

func newPubsubCollector() *pubsubCollector {
	return &pubsubCollector{ch: make(chan actor.BrokerPublishedMessage, 64)}
}

func (c *pubsubCollector) Handle(ctx context.Context, state any, msg actor.Message) (any, error) {
	if pub, ok := msg.Payload.(actor.BrokerPublishedMessage); ok {
		c.mu.Lock()
		c.messages = append(c.messages, pub)
		c.mu.Unlock()
		select {
		case c.ch <- pub:
		default:
		}
	}
	return state, nil
}

func (c *pubsubCollector) waitMessage(t *testing.T, timeout time.Duration) actor.BrokerPublishedMessage {
	t.Helper()
	select {
	case msg := <-c.ch:
		return msg
	case <-time.After(timeout):
		t.Fatal("timeout waiting for pubsub message")
		return actor.BrokerPublishedMessage{}
	}
}

func (c *pubsubCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.messages)
}

func TestPubSub_CrossNodeDirectBroadcast(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register(actor.BrokerPublishedMessage{})
	codec.Register(pubsubBroadcastMsg{})
	codec.Register(actor.BrokerPublishCommand{})
	codec.Register(actor.BrokerSubscribeCommand{})
	codec.Register(actor.BrokerCommandAck{})
	codec.Register(actor.AskReplyEnvelope{})
	codec.Register(actor.BrokerUnsubscribeCommand{})
	codec.Register(brokerRemotePublishCmd{})

	// Build 2-node cluster.
	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	// Create pubsub adapters.
	adapter1 := NewDirectPubSubAdapter(c1, codec)
	adapter2 := NewDirectPubSubAdapter(c2, codec)

	// Create runtimes with pubsub broadcast wired.
	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithPubSubBroadcast(func(ctx context.Context, topic string, payload any) error {
			return adapter1.Broadcast(ctx, topic, payload, "node-1")
		}),
	)
	rt2 := actor.NewRuntime(
		actor.WithNodeID("node-2"),
		actor.WithPubSubBroadcast(func(ctx context.Context, topic string, payload any) error {
			return adapter2.Broadcast(ctx, topic, payload, "node-2")
		}),
	)

	// Wire remote publish handlers — when a broadcast arrives, inject
	// into the local broker without re-broadcasting.
	adapter1.SetHandler(func(topic string, payload any, from NodeID) {
		rt1.BrokerPublishRemote(ctx, "", topic, payload, string(from))
	})
	adapter2.SetHandler(func(topic string, payload any, from NodeID) {
		rt2.BrokerPublishRemote(ctx, "", topic, payload, string(from))
	})

	// Wire inbound envelope routing to the pubsub adapters.
	// Wire via InboundDispatcher + RegisterPubSubHandler.
	d1 := NewInboundDispatcher(rt1, codec)
	d2 := NewInboundDispatcher(rt2, codec)
	RegisterPubSubHandler(d1, adapter1)
	RegisterPubSubHandler(d2, adapter2)

	c1.cfg.OnEnvelope = func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) }
	c2.cfg.OnEnvelope = func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) }

	// Start clusters and connect.
	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := transport1.listener.Addr().String()
	addr2 := transport2.listener.Addr().String()
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Create subscriber actor on node-2.
	collector2 := newPubsubCollector()
	_, err := rt2.CreateActor("subscriber", nil, collector2.Handle)
	if err != nil {
		t.Fatal(err)
	}
	subPID, _ := rt2.IssuePID("node-2", "subscriber")
	time.Sleep(50 * time.Millisecond)

	// Subscribe on node-2 to "orders.#".
	_, err = rt2.BrokerSubscribe(ctx, "", subPID, "orders.#", 2*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	// Publish from node-1 on topic "orders.completed".
	rt1.EnsureBrokerActor("")
	time.Sleep(50 * time.Millisecond)
	rt1.BrokerPublish(ctx, "", "orders.completed", "order-123", "publisher")

	// The subscriber on node-2 should receive it via cross-node broadcast.
	msg := collector2.waitMessage(t, 3*time.Second)
	if msg.Topic != "orders.completed" {
		t.Errorf("topic: got %s, want orders.completed", msg.Topic)
	}
	if msg.Payload != "order-123" {
		t.Errorf("payload: got %v, want order-123", msg.Payload)
	}
}

// brokerRemotePublishCmd is registered for gob since it's used internally.
type brokerRemotePublishCmd = actor.BrokerPublishCommand

func TestPubSub_NoRebroadcastLoop(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register(actor.BrokerPublishedMessage{})
	codec.Register(pubsubBroadcastMsg{})

	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	adapter1 := NewDirectPubSubAdapter(c1, codec)
	adapter2 := NewDirectPubSubAdapter(c2, codec)

	// Count how many times each adapter broadcasts.
	var mu sync.Mutex
	broadcastCount := map[string]int{}

	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithPubSubBroadcast(func(ctx context.Context, topic string, payload any) error {
			mu.Lock()
			broadcastCount["node-1"]++
			mu.Unlock()
			return adapter1.Broadcast(ctx, topic, payload, "node-1")
		}),
	)
	rt2 := actor.NewRuntime(
		actor.WithNodeID("node-2"),
		actor.WithPubSubBroadcast(func(ctx context.Context, topic string, payload any) error {
			mu.Lock()
			broadcastCount["node-2"]++
			mu.Unlock()
			return adapter2.Broadcast(ctx, topic, payload, "node-2")
		}),
	)

	adapter1.SetHandler(func(topic string, payload any, from NodeID) {
		rt1.BrokerPublishRemote(ctx, "", topic, payload, string(from))
	})
	adapter2.SetHandler(func(topic string, payload any, from NodeID) {
		rt2.BrokerPublishRemote(ctx, "", topic, payload, string(from))
	})

	d1 := NewInboundDispatcher(rt1, codec)
	d2 := NewInboundDispatcher(rt2, codec)
	RegisterPubSubHandler(d1, adapter1)
	RegisterPubSubHandler(d2, adapter2)

	c1.cfg.OnEnvelope = func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) }
	c2.cfg.OnEnvelope = func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) }

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := transport1.listener.Addr().String()
	addr2 := transport2.listener.Addr().String()
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Publish from node-1.
	rt1.EnsureBrokerActor("")
	rt2.EnsureBrokerActor("")
	time.Sleep(50 * time.Millisecond)

	rt1.BrokerPublish(ctx, "", "test.topic", "payload", "pub")

	// Wait for delivery.
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	b1 := broadcastCount["node-1"]
	b2 := broadcastCount["node-2"]
	mu.Unlock()

	// Node-1 should broadcast once (originating publish).
	// Node-2 should NOT broadcast (received via remote, no re-broadcast).
	if b1 != 1 {
		t.Errorf("node-1 broadcast count: got %d, want 1", b1)
	}
	if b2 != 0 {
		t.Errorf("node-2 broadcast count: got %d, want 0 (no re-broadcast)", b2)
	}
}

func TestDirectPubSubAdapter_HandleInbound(t *testing.T) {
	codec := NewGobCodec()
	codec.Register(pubsubBroadcastMsg{})
	codec.Register("")

	transport := NewGRPCTransport("n1")
	provider := NewFixedProvider(FixedProviderConfig{})
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "n1", Addr: "127.0.0.1:0"},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: codec,
	})

	adapter := NewDirectPubSubAdapter(c, codec)

	var receivedTopic string
	var receivedPayload any
	adapter.SetHandler(func(topic string, payload any, from NodeID) {
		receivedTopic = topic
		receivedPayload = payload
	})

	// Simulate an inbound envelope.
	payloadBytes, _ := codec.Encode("test-data")
	msg := pubsubBroadcastMsg{
		Topic:         "events.user.created",
		Payload:       payloadBytes,
		PublisherNode: "node-2",
	}
	msgBytes, _ := codec.Encode(msg)

	adapter.HandleInbound(Envelope{
		TypeName: pubsubEnvelopeType,
		Payload:  msgBytes,
	})

	if receivedTopic != "events.user.created" {
		t.Errorf("topic: got %s", receivedTopic)
	}
	if receivedPayload != "test-data" {
		t.Errorf("payload: got %v", receivedPayload)
	}
}
