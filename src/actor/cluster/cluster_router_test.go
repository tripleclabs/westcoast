package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestClusterRouter_JoinAndMembers(t *testing.T) {
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	cr := NewClusterRouter(rt, registry)

	p1 := actor.PID{Namespace: "node-1", ActorID: "worker-1", Generation: 1}
	p2 := actor.PID{Namespace: "node-2", ActorID: "worker-2", Generation: 1}

	cr.Join("payment", p1)
	cr.Join("payment", p2)

	members := cr.Members("payment")
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestClusterRouter_Leave(t *testing.T) {
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	cr := NewClusterRouter(rt, registry)

	p1 := actor.PID{Namespace: "node-1", ActorID: "w1", Generation: 1}
	p2 := actor.PID{Namespace: "node-1", ActorID: "w2", Generation: 1}

	cr.Join("svc", p1)
	cr.Join("svc", p2)
	cr.Leave("svc", p1)

	members := cr.Members("svc")
	if len(members) != 1 {
		t.Fatalf("expected 1 member after leave, got %d", len(members))
	}
	if members[0].ActorID != "w2" {
		t.Errorf("expected w2, got %s", members[0].ActorID)
	}
}

func TestClusterRouter_SendRoundRobin(t *testing.T) {
	ctx := context.Background()
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	// Create two worker actors.
	var mu sync.Mutex
	received := map[string]int{}

	makeWorker := func(id string) {
		rt.CreateActor(id, nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
			mu.Lock()
			received[id]++
			mu.Unlock()
			return state, nil
		})
	}

	makeWorker("w1")
	makeWorker("w2")
	pid1, _ := rt.IssuePID("node-1", "w1")
	pid2, _ := rt.IssuePID("node-1", "w2")

	time.Sleep(50 * time.Millisecond)

	cr := NewClusterRouter(rt, registry)
	cr.Configure("svc", actor.RouterStrategyRoundRobin)
	cr.Join("svc", pid1)
	cr.Join("svc", pid2)

	// Send 10 messages — should distribute evenly.
	for i := 0; i < 10; i++ {
		ack := cr.Send(ctx, "svc", "msg")
		if ack.Outcome != actor.PIDDelivered {
			t.Fatalf("send %d: %s", i, ack.Outcome)
		}
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	r1 := received["w1"]
	r2 := received["w2"]
	mu.Unlock()

	if r1 != 5 || r2 != 5 {
		t.Errorf("expected 5/5 distribution, got w1=%d w2=%d", r1, r2)
	}
}

func TestClusterRouter_SendNoWorkers(t *testing.T) {
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	cr := NewClusterRouter(rt, registry)

	ack := cr.Send(context.Background(), "empty-service", "payload")
	if ack.Outcome != actor.PIDRejectedNotFound {
		t.Errorf("expected not found, got %s", ack.Outcome)
	}
}

func TestClusterRouter_Broadcast(t *testing.T) {
	ctx := context.Background()
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	var mu sync.Mutex
	received := map[string]int{}

	for _, id := range []string{"w1", "w2", "w3"} {
		id := id
		rt.CreateActor(id, nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
			mu.Lock()
			received[id]++
			mu.Unlock()
			return state, nil
		})
	}

	pid1, _ := rt.IssuePID("node-1", "w1")
	pid2, _ := rt.IssuePID("node-1", "w2")
	pid3, _ := rt.IssuePID("node-1", "w3")

	time.Sleep(50 * time.Millisecond)

	cr := NewClusterRouter(rt, registry)
	cr.Join("bcast", pid1)
	cr.Join("bcast", pid2)
	cr.Join("bcast", pid3)

	acks := cr.Broadcast(ctx, "bcast", "hello-all")
	if len(acks) != 3 {
		t.Fatalf("expected 3 acks, got %d", len(acks))
	}
	for i, ack := range acks {
		if ack.Outcome != actor.PIDDelivered {
			t.Errorf("ack[%d]: %s", i, ack.Outcome)
		}
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	for _, id := range []string{"w1", "w2", "w3"} {
		if received[id] != 1 {
			t.Errorf("%s received %d, want 1", id, received[id])
		}
	}
}

func TestClusterRouter_MembersIsolated(t *testing.T) {
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	cr := NewClusterRouter(rt, registry)

	cr.Join("svc-a", actor.PID{Namespace: "n1", ActorID: "a1", Generation: 1})
	cr.Join("svc-b", actor.PID{Namespace: "n1", ActorID: "b1", Generation: 1})

	if len(cr.Members("svc-a")) != 1 {
		t.Error("svc-a should have 1 member")
	}
	if len(cr.Members("svc-b")) != 1 {
		t.Error("svc-b should have 1 member")
	}
	if len(cr.Members("svc-c")) != 0 {
		t.Error("svc-c should have 0 members")
	}
}

func TestClusterRouter_CrossNode(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

	transport1 := NewTCPTransport("node-1")
	transport2 := NewTCPTransport("node-2")
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

	remoteSender1 := NewRemoteSender(c1, codec, nil)

	// Worker on node-2.
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	collector := newCollectingHandler()
	rt2.CreateActor("worker", nil, collector.Handle)
	workerPID, _ := rt2.IssuePID("node-2", "worker")

	// Router on node-1 with the remote worker.
	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithRemoteSend(remoteSender1.Send),
	)

	// Shared registry.
	registry := NewCRDTRegistry("shared")
	cr := NewClusterRouter(rt1, registry)
	cr.Join("my-service", workerPID)

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

	// Route from node-1 to worker on node-2 via cluster router.
	ack := cr.Send(ctx, "my-service", "routed-cross-node")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector.waitMessage(t, 2*time.Second)
	if msg != "routed-cross-node" {
		t.Errorf("got %v", msg)
	}
}
