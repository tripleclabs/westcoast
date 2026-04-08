package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestCronDaemon_StartsLocalCronActor(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	dm.RegisterCronDaemon("", nil)
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	// The cron timer daemon should be running.
	running := dm.Running()
	found := false
	for _, name := range running {
		if name == actor.DefaultCronActorID {
			found = true
		}
	}
	if !found {
		t.Fatalf("cron daemon not running; running: %v", running)
	}

	if rt.Status(actor.DefaultCronActorID) != actor.ActorRunning {
		t.Fatal("cron daemon actor should be running")
	}
}

func TestCronDaemon_LocalSubscription(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	dm.RegisterCronDaemon("", nil)
	dm.Start(context.Background())
	defer dm.Stop()

	// Create a subscriber actor.
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	// Subscribe via the convenience API (uses the daemon's cron actor).
	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "@every 500ms", "daemon-tick", "test", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}

	tick := collector.waitTick(t, 3*time.Second)
	if tick.Payload != "daemon-tick" {
		t.Errorf("expected 'daemon-tick', got %v", tick.Payload)
	}
}

func TestCronDaemon_CustomID(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	dm := NewDaemonSetManager(rt, nil, nil)

	dm.RegisterCronDaemon("my-cron-timer", nil)
	dm.Start(context.Background())
	defer dm.Stop()

	time.Sleep(50 * time.Millisecond)

	if rt.Status("my-cron-timer") != actor.ActorRunning {
		t.Fatal("custom cron daemon should be running")
	}

	// Subscribe using the custom ID.
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "my-cron-timer", pid, "@every 500ms", "custom", "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected success, got %s", ack.Result)
	}

	tick := collector.waitTick(t, 3*time.Second)
	if tick.Payload != "custom" {
		t.Errorf("expected 'custom', got %v", tick.Payload)
	}
}
