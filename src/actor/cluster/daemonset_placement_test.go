package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestDaemonSet_PlacementMatchStarts(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "4"}},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	dm := NewDaemonSetManager(rt, c, nil)
	dm.Register(DaemonSpec{
		Name:      "gpu-worker",
		Handler:   nop,
		Placement: TagGTE("gpus", 1),
	})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	running := dm.Running()
	if len(running) != 1 || running[0] != "gpu-worker" {
		t.Errorf("expected gpu-worker running, got %v", running)
	}
}

func TestDaemonSet_PlacementNoMatchSkips(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "0"}},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	dm := NewDaemonSetManager(rt, c, nil)
	dm.Register(DaemonSpec{
		Name:      "gpu-worker",
		Handler:   nop,
		Placement: TagGTE("gpus", 1),
	})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	if len(dm.Running()) != 0 {
		t.Error("gpu-worker should NOT run on a node with 0 GPUs")
	}
}

func TestDaemonSet_PlacementReactiveStart(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{}},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	dm := NewDaemonSetManager(rt, c, nil)
	dm.Register(DaemonSpec{
		Name:      "gpu-worker",
		Handler:   nop,
		Placement: TagExists("gpus"),
	})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should not be running — no gpus tag.
	if len(dm.Running()) != 0 {
		t.Fatal("should not run without gpus tag")
	}

	// Node gets GPUs — update tags and reconcile.
	c.UpdateTags(map[string]string{"gpus": "2"})
	dm.Reconcile()

	time.Sleep(50 * time.Millisecond)

	if len(dm.Running()) != 1 {
		t.Error("should start after gaining gpus tag")
	}
}

func TestDaemonSet_PlacementReactiveStop(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"gpus": "4"}},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	dm := NewDaemonSetManager(rt, c, nil)
	dm.Register(DaemonSpec{
		Name:      "gpu-worker",
		Handler:   nop,
		Placement: TagGTE("gpus", 1),
	})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)
	if len(dm.Running()) != 1 {
		t.Fatal("should be running with 4 GPUs")
	}

	// GPUs fail — tag goes to 0.
	c.UpdateTags(map[string]string{"gpus": "0"})
	dm.Reconcile()

	time.Sleep(50 * time.Millisecond)

	if len(dm.Running()) != 0 {
		t.Error("should stop after losing GPUs")
	}
}

func TestDaemonSet_PlacementNilRunsEverywhere(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }

	dm := NewDaemonSetManager(rt, nil, nil)
	dm.Register(DaemonSpec{
		Name:    "everywhere",
		Handler: nop,
		// Placement: nil — runs on all nodes
	})
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	if len(dm.Running()) != 1 {
		t.Error("nil placement should run everywhere")
	}
}

func TestDaemonSet_BroadcastRespectsPlacement(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"role": "gpu"}},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"role": "cpu"}},
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

	addr2 := transport2.listener.Addr().String()
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2, Tags: map[string]string{"role": "cpu"}})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout")
		case <-time.After(20 * time.Millisecond):
		}
	}

	collector1 := newCollectingHandler()

	dm1 := NewDaemonSetManager(rt1, c1, codec)
	dm1.Register(DaemonSpec{
		Name:      "gpu-daemon",
		Handler:   collector1.Handle,
		Placement: TagEquals("role", "gpu"),
	})
	dm1.Start(ctx)
	defer dm1.Stop()

	// Node-2 also registers but shouldn't run (role=cpu, not gpu).
	dm2 := NewDaemonSetManager(rt2, c2, codec)
	dm2.Register(DaemonSpec{
		Name:      "gpu-daemon",
		Handler:   func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil },
		Placement: TagEquals("role", "gpu"),
	})
	dm2.Start(ctx)
	defer dm2.Stop()

	time.Sleep(50 * time.Millisecond)

	// Only node-1 should run the daemon.
	if len(dm1.Running()) != 1 {
		t.Errorf("node-1 (gpu) should run: %v", dm1.Running())
	}
	if len(dm2.Running()) != 0 {
		t.Errorf("node-2 (cpu) should NOT run: %v", dm2.Running())
	}

	// Broadcast should only target node-1 (the matching node).
	acks := dm1.Broadcast(ctx, "gpu-daemon", "gpu-only-msg")
	if len(acks) != 1 {
		t.Errorf("broadcast should target 1 node, got %d", len(acks))
	}

	collector1.waitMessage(t, 2*time.Second)
}
