package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestDaemonSet_StartsLocalActors(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	nopHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	dm.Register(DaemonSpec{Name: "daemon-a", Handler: nopHandler})
	dm.Register(DaemonSpec{Name: "daemon-b", Handler: nopHandler})
	dm.Register(DaemonSpec{Name: "daemon-c", Handler: nopHandler})

	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	running := dm.Running()
	if len(running) != 3 {
		t.Fatalf("expected 3 running, got %d: %v", len(running), running)
	}

	for _, name := range []string{"daemon-a", "daemon-b", "daemon-c"} {
		if rt.Status(name) != actor.ActorRunning {
			t.Errorf("%s should be running", name)
		}
	}
}

func TestDaemonSet_SendTo_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	collector := newCollectingHandler()
	dm.Register(DaemonSpec{Name: "collector", Handler: collector.Handle})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	ack := dm.SendTo(context.Background(), "collector", "node-1", "hello-daemon")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector.waitMessage(t, 2*time.Second)
	if msg != "hello-daemon" {
		t.Errorf("got %v", msg)
	}
}

func TestDaemonSet_AskTo_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	echoHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		if replyTo, ok := msg.AskReplyTo(); ok {
			rt.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   "echo:" + msg.Payload.(string),
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	}

	dm.Register(DaemonSpec{Name: "echo", Handler: echoHandler})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	result, err := dm.AskTo(context.Background(), "echo", "node-1", "ping", 2*time.Second)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if result.Payload != "echo:ping" {
		t.Errorf("got %v", result.Payload)
	}
}

func TestDaemonSet_SendTo_Remote(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

	transport1 := newTestTransport("node-1")
	transport2 := newTestTransport("node-2")
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

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))

	// Inbound dispatcher for node-2.
	dispatcher2 := NewInboundDispatcher(rt2, codec)
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { dispatcher2.Dispatch(ctx, from, env) })

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr2 := testTransportAddr(transport2)
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Register daemons on both nodes.
	collector2 := newCollectingHandler()

	dm1 := NewDaemonSetManager(rt1, c1, codec)
	dm1.Register(DaemonSpec{Name: "daemon", Handler: func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }})
	dm1.Start(ctx)
	defer dm1.Stop()

	dm2 := NewDaemonSetManager(rt2, c2, codec)
	dm2.Register(DaemonSpec{Name: "daemon", Handler: collector2.Handle})
	dm2.Start(ctx)
	defer dm2.Stop()

	time.Sleep(50 * time.Millisecond)

	// Send from node-1's daemon manager to daemon on node-2.
	ack := dm1.SendTo(ctx, "daemon", "node-2", "cross-node-daemon")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector2.waitMessage(t, 2*time.Second)
	if msg != "cross-node-daemon" {
		t.Errorf("got %v", msg)
	}
}

func TestDaemonSet_AskTo_Remote(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")
	codec.Register(actor.AskReplyEnvelope{})

	transport1 := newTestTransport("node-1")
	transport2 := newTestTransport("node-2")
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

	sender1 := NewRemoteSender(c1, codec, nil)
	sender2 := NewRemoteSender(c2, codec, nil)

	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithRemoteSend(sender1.Send),
		actor.WithRemoteAskSend(sender1.SendAsk),
	)
	rt2 := actor.NewRuntime(
		actor.WithNodeID("node-2"),
		actor.WithRemoteSend(sender2.Send),
	)

	// Wire inbound dispatchers.
	d1 := NewInboundDispatcher(rt1, codec)
	d2 := NewInboundDispatcher(rt2, codec)
	c1.SetOnEnvelope(func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) })
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := testTransportAddr(transport1)
	addr2 := testTransportAddr(transport2)
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") || !c2.IsConnected("node-1") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Echo daemon on node-2.
	receivedAsk := make(chan string, 4)
	echoHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		receivedAsk <- "got"
		if replyTo, ok := msg.AskReplyTo(); ok {
			rt2.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   "echo:" + msg.Payload.(string),
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	}

	dm1 := NewDaemonSetManager(rt1, c1, codec)
	dm1.Register(DaemonSpec{Name: "echo-daemon", Handler: func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }})
	dm1.Start(ctx)
	defer dm1.Stop()

	dm2 := NewDaemonSetManager(rt2, c2, codec)
	dm2.Register(DaemonSpec{Name: "echo-daemon", Handler: echoHandler})
	dm2.Start(ctx)
	defer dm2.Stop()

	time.Sleep(50 * time.Millisecond)

	// First verify fire-and-forget works.
	sendAck := dm1.SendTo(ctx, "echo-daemon", "node-2", "test-fire-forget")
	if sendAck.Outcome != actor.PIDDelivered {
		t.Fatalf("SendTo failed: %s", sendAck.Outcome)
	}
	select {
	case <-receivedAsk:
		t.Log("fire-and-forget confirmed working")
	case <-time.After(2 * time.Second):
		t.Fatal("fire-and-forget not received")
	}

	result, err := dm1.AskTo(ctx, "echo-daemon", "node-2", "hello", 3*time.Second)
	if err != nil {
		t.Fatalf("ask: %v", err)
	}
	if result.Payload != "echo:hello" {
		t.Errorf("got %v", result.Payload)
	}
}

func TestDaemonSet_Broadcast(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

	transport1 := newTestTransport("node-1")
	transport2 := newTestTransport("node-2")
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

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))

	d2 := NewInboundDispatcher(rt2, codec)
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr2 := testTransportAddr(transport2)
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	collector1 := newCollectingHandler()
	collector2 := newCollectingHandler()

	dm1 := NewDaemonSetManager(rt1, c1, codec)
	dm1.Register(DaemonSpec{Name: "bcast", Handler: collector1.Handle})
	dm1.Start(ctx)
	defer dm1.Stop()

	dm2 := NewDaemonSetManager(rt2, c2, codec)
	dm2.Register(DaemonSpec{Name: "bcast", Handler: collector2.Handle})
	dm2.Start(ctx)
	defer dm2.Stop()

	time.Sleep(50 * time.Millisecond)

	acks := dm1.Broadcast(ctx, "bcast", "hello-all")

	if len(acks) != 2 {
		t.Fatalf("expected 2 acks, got %d", len(acks))
	}
	for i, ack := range acks {
		if ack.Outcome != actor.PIDDelivered {
			t.Errorf("ack[%d]: %s", i, ack.Outcome)
		}
	}

	// Both should receive the message.
	collector1.waitMessage(t, 2*time.Second)
	collector2.waitMessage(t, 2*time.Second)
}

func TestDaemonSet_Stop(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }
	dm.Register(DaemonSpec{Name: "d1", Handler: nop})
	dm.Register(DaemonSpec{Name: "d2", Handler: nop})

	dm.Start(context.Background())
	time.Sleep(50 * time.Millisecond)

	if len(dm.Running()) != 2 {
		t.Fatal("should have 2 running")
	}

	dm.Stop()

	if len(dm.Running()) != 0 {
		t.Error("should have 0 running after stop")
	}
	for _, name := range []string{"d1", "d2"} {
		if rt.Status(name) != actor.ActorStopped {
			t.Errorf("%s should be stopped", name)
		}
	}
}

func TestDaemonSet_DrainStopsDaemons(t *testing.T) {
	ctx := context.Background()
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }
	dm := NewDaemonSetManager(rt, nil, nil)
	dm.Register(DaemonSpec{Name: "drain-daemon", Handler: nop})
	dm.Start(ctx)

	time.Sleep(50 * time.Millisecond)
	if len(dm.Running()) != 1 {
		t.Fatal("should be running before drain")
	}

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := newTestTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: NewGobCodec(),
	})
	c.Start(ctx)

	Drain(ctx, c, DrainConfig{Timeout: 1 * time.Second}, WithDaemonSetManager(dm))

	if len(dm.Running()) != 0 {
		t.Error("daemons should be stopped after drain")
	}
}

func TestDaemonSet_SingleNodeWorks(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("solo"))
	dm := NewDaemonSetManager(rt, nil, nil)

	collector := newCollectingHandler()
	echoHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		if replyTo, ok := msg.AskReplyTo(); ok {
			rt.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   "echo:" + msg.Payload.(string),
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	}

	dm.Register(DaemonSpec{Name: "collector", Handler: collector.Handle})
	dm.Register(DaemonSpec{Name: "echo", Handler: echoHandler})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	// SendTo local.
	ack := dm.SendTo(context.Background(), "collector", "solo", "single-node")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("SendTo: %s", ack.Outcome)
	}
	collector.waitMessage(t, 2*time.Second)

	// AskTo local.
	result, err := dm.AskTo(context.Background(), "echo", "solo", "ping", 2*time.Second)
	if err != nil {
		t.Fatalf("AskTo: %v", err)
	}
	if result.Payload != "echo:ping" {
		t.Errorf("AskTo: got %v", result.Payload)
	}

	// Broadcast (single node = 1 ack).
	acks := dm.Broadcast(context.Background(), "collector", "broadcast-msg")
	if len(acks) != 1 {
		t.Errorf("Broadcast: expected 1 ack, got %d", len(acks))
	}
}
