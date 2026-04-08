package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestCronSingleton_SingleNode(t *testing.T) {
	ctx := context.Background()
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	p := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	c, err := Start(ctx, rt, Config{
		Addr:     "127.0.0.1:0",
		Provider: p,
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	c.Singletons().RegisterCronSingleton("", nil, c.cfg.Codec)

	time.Sleep(500 * time.Millisecond)

	// Verify the singleton cron actor is running.
	running := c.Singletons().Running()
	found := false
	for _, name := range running {
		if name == actor.SingletonCronActorID {
			found = true
		}
	}
	if !found {
		t.Fatalf("singleton cron not running; running: %v", running)
	}

	// Create a subscriber and subscribe.
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ack, err := rt.CronSubscribe(ctx, actor.SingletonCronActorID, pid, "@every 500ms", "singleton-tick", "test", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}

	tick := collector.waitTick(t, 3*time.Second)
	if tick.Payload != "singleton-tick" {
		t.Errorf("expected 'singleton-tick', got %v", tick.Payload)
	}
}

func TestCronSingleton_TwoNode_StartsOnLeader(t *testing.T) {
	ctx := context.Background()

	// --- Node 1 ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c1, err := Start(ctx, rt1, Config{
		Addr:     "127.0.0.1:0",
		Provider: p1,
	})
	if err != nil {
		t.Fatalf("Start node-1: %v", err)
	}
	defer c1.Stop()
	addr1 := testTransportAddr(c1.cfg.Transport)

	c1.Singletons().RegisterCronSingleton("", nil, c1.cfg.Codec)
	time.Sleep(500 * time.Millisecond)

	// Node 1 should be running the singleton.
	r1 := c1.Singletons().Running()
	found := false
	for _, name := range r1 {
		if name == actor.SingletonCronActorID {
			found = true
		}
	}
	if !found {
		t.Fatalf("singleton cron not on node-1; running: %v", r1)
	}

	// Subscribe a local worker.
	collector := newCronCollector()
	rt1.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt1.IssuePID("node-1", "worker")
	time.Sleep(50 * time.Millisecond)

	ack, err := rt1.CronSubscribe(ctx, actor.SingletonCronActorID, pid, "@every 500ms", "tick", "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected success, got %s", ack.Result)
	}

	// Verify ticks are arriving.
	collector.waitTick(t, 3*time.Second)

	// --- Node 2 ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 100 * time.Millisecond,
	})

	c2, err := Start(ctx, rt2, Config{
		Addr:     "127.0.0.1:0",
		Provider: p2,
	})
	if err != nil {
		t.Fatalf("Start node-2: %v", err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)
	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	c2.Singletons().RegisterCronSingleton("", nil, c2.cfg.Codec)
	time.Sleep(3 * time.Second)

	// The singleton should still be on exactly one node.
	r1 = c1.Singletons().Running()
	r2 := c2.Singletons().Running()

	hasCron1 := false
	hasCron2 := false
	for _, name := range r1 {
		if name == actor.SingletonCronActorID {
			hasCron1 = true
		}
	}
	for _, name := range r2 {
		if name == actor.SingletonCronActorID {
			hasCron2 = true
		}
	}

	// Exactly one node should have it (singletons are sticky).
	if hasCron1 == hasCron2 {
		t.Errorf("singleton cron should be on exactly one node: node-1=%v node-2=%v", hasCron1, hasCron2)
	}

	t.Logf("singleton cron: node-1=%v node-2=%v", hasCron1, hasCron2)
}
