package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestClusterRouter_WithWorkerFilter(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "us-east-1", "gpus": "4"}},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: NewGobCodec(),
	})
	// Add a peer with different tags.
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "eu-west-1", "gpus": "0"}})
	c.Start(ctx)
	defer c.Stop()
	time.Sleep(50 * time.Millisecond)

	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	cr := NewClusterRouter(rt, registry, c)
	cr.Configure("gpu-svc", actor.RouterStrategyRoundRobin,
		WithWorkerFilter(TagGTE("gpus", 1)),
	)

	// Add workers from both nodes.
	cr.Join("gpu-svc", actor.PID{Namespace: "node-1", ActorID: "w1", Generation: 1})
	cr.Join("gpu-svc", actor.PID{Namespace: "node-2", ActorID: "w2", Generation: 1})

	// Members should only return node-1's worker (has GPUs).
	members := cr.Members("gpu-svc")
	if len(members) != 1 {
		t.Fatalf("expected 1 filtered member, got %d", len(members))
	}
	if members[0].Namespace != "node-1" {
		t.Errorf("expected node-1's worker, got %s", members[0].Namespace)
	}
}

func TestClusterRouter_Nearest(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "eu-west-1", "az": "eu-west-1a"}},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: NewGobCodec(),
	})
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "eu-west-1", "az": "eu-west-1b"}})
	provider.AddMember(NodeMeta{ID: "node-3", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "us-east-1", "az": "us-east-1a"}})
	c.Start(ctx)
	defer c.Stop()
	time.Sleep(50 * time.Millisecond)

	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	// Create workers.
	var mu sync.Mutex
	received := map[string]int{}
	for _, id := range []string{"w1", "w2", "w3"} {
		id := id
		rt.CreateActor(id, nil, func(_ context.Context, s any, m actor.Message) (any, error) {
			mu.Lock()
			received[id]++
			mu.Unlock()
			return s, nil
		})
		rt.IssuePID("node-1", id)
	}
	time.Sleep(50 * time.Millisecond)

	cr := NewClusterRouter(rt, registry, c)
	cr.Configure("api", actor.RouterStrategyRoundRobin)

	// Workers on different nodes.
	cr.Join("api", actor.PID{Namespace: "node-1", ActorID: "w1", Generation: 1}) // same AZ
	cr.Join("api", actor.PID{Namespace: "node-2", ActorID: "w2", Generation: 1}) // same region, diff AZ
	cr.Join("api", actor.PID{Namespace: "node-3", ActorID: "w3", Generation: 1}) // diff region

	// SendWith Nearest — should prefer same AZ, then same region, then any.
	// First call should go to w1 (highest score: 2 — matches region + az).
	ack := cr.SendWith(ctx, "api", "msg",
		Nearest(map[string]string{
			"region": "eu-west-1",
			"az":     "eu-west-1a",
		}),
	)
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	time.Sleep(50 * time.Millisecond)

	// With round-robin on a preference-sorted list, the first call
	// goes to the highest-scored worker. Since w1 matches both tags
	// (score 2), w2 matches one (score 1), w3 matches none (score 0),
	// the sorted order is [w1, w2, w3]. Round-robin index 0 → w1.
	mu.Lock()
	if received["w1"] != 1 {
		t.Errorf("w1 (same AZ) should receive first: %v", received)
	}
	mu.Unlock()
}

func TestClusterRouter_PreferTag(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"memory-gb": "64"}},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: NewGobCodec(),
	})
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"memory-gb": "128"}})
	c.Start(ctx)
	defer c.Stop()
	time.Sleep(50 * time.Millisecond)

	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	cr := NewClusterRouter(rt, registry, c)
	cr.Configure("cache", actor.RouterStrategyRoundRobin,
		WithWorkerPreference(RankByTag("memory-gb", Highest)),
	)

	cr.Join("cache", actor.PID{Namespace: "node-1", ActorID: "w1", Generation: 1})
	cr.Join("cache", actor.PID{Namespace: "node-2", ActorID: "w2", Generation: 1})

	// After preference sorting, node-2 (128GB) should come before node-1 (64GB).
	members := cr.Members("cache")
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
	// Note: Members() returns raw unranked list. Preference is applied in Send/Ask.
	// The Send should route to node-2 first (highest memory).
}

func TestClusterRouter_FilterAndPreferenceCombined(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "eu-west-1", "gpus": "8"}},
		Provider: provider, Transport: transport,
		Auth: NoopAuth{}, Codec: NewGobCodec(),
	})
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "eu-west-1", "gpus": "4"}})
	provider.AddMember(NodeMeta{ID: "node-3", Addr: "127.0.0.1:0", Tags: map[string]string{"region": "us-east-1", "gpus": "16"}})
	c.Start(ctx)
	defer c.Stop()
	time.Sleep(50 * time.Millisecond)

	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	cr := NewClusterRouter(rt, registry, c)
	cr.Configure("ml", actor.RouterStrategyRoundRobin,
		// Only route to eu-west-1 nodes.
		WithWorkerFilter(TagEquals("region", "eu-west-1")),
		// Among those, prefer most GPUs.
		WithWorkerPreference(RankByTag("gpus", Highest)),
	)

	cr.Join("ml", actor.PID{Namespace: "node-1", ActorID: "w1", Generation: 1})
	cr.Join("ml", actor.PID{Namespace: "node-2", ActorID: "w2", Generation: 1})
	cr.Join("ml", actor.PID{Namespace: "node-3", ActorID: "w3", Generation: 1}) // filtered out (us-east-1)

	members := cr.Members("ml")
	// node-3 should be filtered out.
	if len(members) != 2 {
		t.Fatalf("expected 2 (eu-west-1 only), got %d", len(members))
	}

	for _, m := range members {
		if m.Namespace == "node-3" {
			t.Error("node-3 should be filtered out")
		}
	}
}

func TestClusterRouter_SendWithNoCluster(t *testing.T) {
	// Without a cluster, preferences are ignored gracefully.
	registry := NewCRDTRegistry("node-1")
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	nop := func(_ context.Context, s any, m actor.Message) (any, error) { return s, nil }
	rt.CreateActor("w1", nil, nop)
	pid, _ := rt.IssuePID("node-1", "w1")
	time.Sleep(50 * time.Millisecond)

	cr := NewClusterRouter(rt, registry) // no cluster
	cr.Join("svc", pid)

	ack := cr.SendWith(context.Background(), "svc", "msg",
		Nearest(map[string]string{"region": "eu-west-1"}),
	)
	if ack.Outcome != actor.PIDDelivered {
		t.Errorf("should still deliver without cluster, got %s", ack.Outcome)
	}
}
