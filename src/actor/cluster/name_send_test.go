package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestSendName_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	collector := newCollectingHandler()
	_, err := rt.CreateActor("svc", nil, collector.Handle)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := rt.IssuePID("node-1", "svc")
	rt.RegisterName("svc", "my-service", "node-1")
	_ = pid

	time.Sleep(50 * time.Millisecond)

	ack := rt.SendName(context.Background(), "my-service", "hello-by-name")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector.waitMessage(t, 2*time.Second)
	if msg != "hello-by-name" {
		t.Errorf("got %v", msg)
	}
}

func TestSendName_NotFound(t *testing.T) {
	rt := actor.NewRuntime()

	ack := rt.SendName(context.Background(), "nonexistent", "payload")
	if ack.Outcome != actor.PIDRejectedNotFound {
		t.Errorf("expected not found, got %s", ack.Outcome)
	}
}

func TestAskName_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	// Echo handler that replies to asks.
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		if replyTo, ok := msg.AskReplyTo(); ok {
			rt.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   "echo:" + msg.Payload.(string),
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	}

	rt.CreateActor("echo", nil, handler)
	rt.IssuePID("node-1", "echo")
	rt.RegisterName("echo", "echo-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	result, err := rt.AskName(context.Background(), "echo-service", "ping", 2*time.Second)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if result.Payload != "echo:ping" {
		t.Errorf("got %v", result.Payload)
	}
}

func TestAskName_NotFound(t *testing.T) {
	rt := actor.NewRuntime()

	_, err := rt.AskName(context.Background(), "nonexistent", "payload", 1*time.Second)
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
}

func TestSendName_CrossNode(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	remoteSender1 := NewRemoteSender(c1, codec, nil)

	// Create rt2 with an echo actor and register its name.
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	collector := newCollectingHandler()
	rt2.CreateActor("echo", nil, collector.Handle)
	pid2, _ := rt2.IssuePID("node-2", "echo")

	// Create a shared CRDT registry and register node-2's actor.
	registry := NewCRDTRegistry("shared")
	registry.Register("remote-echo", pid2)

	// Create rt1 with cluster wiring + cluster registry for lookups.
	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithRemoteSend(remoteSender1.Send),
		actor.WithClusterRegistry(
			func(name string, pid actor.PID) error { return registry.Register(name, pid) },
			func(name string) (actor.PID, bool) { return registry.Lookup(name) },
			func(name string) (actor.PID, bool) { return registry.Unregister(name) },
		),
	)

	// Wire inbound for node-2.
	dispatcher2 := NewInboundDispatcher(rt2, codec)
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { dispatcher2.Dispatch(ctx, from, env) })

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr2 := transport2.listener.Addr().String()
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	time.Sleep(50 * time.Millisecond)

	// SendName from node-1 to "remote-echo" which lives on node-2.
	ack := rt1.SendName(ctx, "remote-echo", "cross-node-by-name")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector.waitMessage(t, 2*time.Second)
	if msg != "cross-node-by-name" {
		t.Errorf("got %v", msg)
	}
}
