package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// cronCollector is a test handler that collects CronTickMessages.
type cronCollector struct {
	mu       sync.Mutex
	messages []actor.CronTickMessage
	ch       chan actor.CronTickMessage
}

func newCronCollector() *cronCollector {
	return &cronCollector{ch: make(chan actor.CronTickMessage, 64)}
}

func (c *cronCollector) Handle(ctx context.Context, state any, msg actor.Message) (any, error) {
	if tick, ok := msg.Payload.(actor.CronTickMessage); ok {
		c.mu.Lock()
		c.messages = append(c.messages, tick)
		c.mu.Unlock()
		select {
		case c.ch <- tick:
		default:
		}
	}
	return state, nil
}

func (c *cronCollector) waitTick(t *testing.T, timeout time.Duration) actor.CronTickMessage {
	t.Helper()
	select {
	case tick := <-c.ch:
		return tick
	case <-time.After(timeout):
		t.Fatal("timeout waiting for cron tick")
		return actor.CronTickMessage{}
	}
}

func (c *cronCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.messages)
}

func TestCronSubscribe_ReceivesTicks(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick-payload", "my-tag", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}
	if ack.Ref.ID == "" {
		t.Fatal("expected non-empty ref ID")
	}
	if ack.Ref.ActorID != "worker" {
		t.Errorf("expected ref actor ID 'worker', got %q", ack.Ref.ActorID)
	}

	// Wait for at least one tick (every second).
	tick := collector.waitTick(t, 3*time.Second)
	if tick.Payload != "tick-payload" {
		t.Errorf("expected payload 'tick-payload', got %v", tick.Payload)
	}
	if tick.Tag != "my-tag" {
		t.Errorf("expected tag 'my-tag', got %q", tick.Tag)
	}
	if tick.Ref.ID != ack.Ref.ID {
		t.Errorf("ref ID mismatch: %q vs %q", tick.Ref.ID, ack.Ref.ID)
	}
}

func TestCronSubscribe_InvalidSchedule(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "not a cron expression", nil, "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeInvalidSchedule {
		t.Errorf("expected invalid_schedule, got %s", ack.Result)
	}
}

func TestCronUnsubscribe_StopsTicks(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, _ := rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick", "", 5*time.Second)

	// Wait for at least one tick.
	collector.waitTick(t, 3*time.Second)

	// Unsubscribe.
	unsub, err := rt.CronUnsubscribe(ctx, "", pid, ack.Ref.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if unsub.Result != actor.CronOutcomeUnsubscribeSuccess {
		t.Errorf("expected unsubscribe_success, got %s", unsub.Result)
	}

	// Drain channel and wait — no more ticks should arrive.
	time.Sleep(100 * time.Millisecond)
	countBefore := collector.count()
	time.Sleep(2 * time.Second)
	countAfter := collector.count()
	if countAfter != countBefore {
		t.Errorf("ticks arrived after unsubscribe: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestCronUnsubscribe_NotFound(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronUnsubscribe(ctx, "", pid, "nonexistent-ref", 5*time.Second)
	if err != nil {
		t.Fatalf("unsubscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeNotFound {
		t.Errorf("expected not_found, got %s", ack.Result)
	}
}

func TestCronMultipleSubscriptions(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack1, _ := rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick-A", "A", 5*time.Second)
	ack2, _ := rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick-B", "B", 5*time.Second)

	if ack1.Ref.ID == ack2.Ref.ID {
		t.Fatal("expected different ref IDs for different subscriptions")
	}

	// Wait for ticks from both.
	seen := map[string]bool{}
	deadline := time.After(5 * time.Second)
	for len(seen) < 2 {
		select {
		case tick := <-collector.ch:
			seen[tick.Tag] = true
		case <-deadline:
			t.Fatalf("timeout; only saw tags: %v", seen)
		}
	}
}

func TestCronUnsubscribeAll(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick-1", "", 5*time.Second)
	rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick-2", "", 5*time.Second)

	// Wait for at least one tick.
	collector.waitTick(t, 3*time.Second)

	// Unsubscribe all.
	ack, err := rt.CronUnsubscribeAll(ctx, "", pid, 5*time.Second)
	if err != nil {
		t.Fatalf("unsubscribe_all: %v", err)
	}
	if ack.Result != actor.CronOutcomeUnsubscribeSuccess {
		t.Errorf("expected unsubscribe_success, got %s", ack.Result)
	}

	time.Sleep(100 * time.Millisecond)
	countBefore := collector.count()
	time.Sleep(2 * time.Second)
	countAfter := collector.count()
	if countAfter != countBefore {
		t.Errorf("ticks arrived after unsubscribe_all: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestCronList(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	sub1, _ := rt.CronSubscribe(ctx, "", pid, "* * * * * *", nil, "A", 5*time.Second)
	sub2, _ := rt.CronSubscribe(ctx, "", pid, "*/2 * * * * *", nil, "B", 5*time.Second)

	list, err := rt.CronList(ctx, "", pid, 5*time.Second)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if list.Result != actor.CronOutcomeListSuccess {
		t.Errorf("expected list_success, got %s", list.Result)
	}
	if len(list.Refs) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(list.Refs))
	}

	refIDs := map[string]bool{}
	for _, ref := range list.Refs {
		refIDs[ref.ID] = true
	}
	if !refIDs[sub1.Ref.ID] || !refIDs[sub2.Ref.ID] {
		t.Errorf("list refs don't match subscriptions: %v", refIDs)
	}
}

func TestCronMonitorCleanup_ActorStop(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	rt.CronSubscribe(ctx, "", pid, "* * * * * *", "tick", "", 5*time.Second)

	// Wait for a tick to confirm it's working.
	collector.waitTick(t, 3*time.Second)

	// Stop the subscriber actor.
	rt.Stop("worker")

	// Give the monitor time to fire and clean up.
	time.Sleep(500 * time.Millisecond)

	// List should return empty for a new actor with the same name,
	// or the cron service internal state should be clean.
	// We verify by checking no more ticks arrive.
	countBefore := collector.count()
	time.Sleep(2 * time.Second)
	countAfter := collector.count()
	if countAfter != countBefore {
		t.Errorf("ticks arrived after actor stop: before=%d, after=%d", countBefore, countAfter)
	}
}

func TestCronLazyAutoStart(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	// Don't call EnsureCronActor — CronSubscribe should auto-create it.
	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "* * * * * *", "auto-start", "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}

	tick := collector.waitTick(t, 3*time.Second)
	if tick.Payload != "auto-start" {
		t.Errorf("expected 'auto-start', got %v", tick.Payload)
	}
}

func TestCronEveryShorthand(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCronCollector()
	rt.CreateActor("worker", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "@every 500ms", "fast-tick", "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}

	// Should receive multiple ticks quickly.
	tick := collector.waitTick(t, 2*time.Second)
	if tick.Payload != "fast-tick" {
		t.Errorf("expected 'fast-tick', got %v", tick.Payload)
	}

	// Wait for a few more.
	time.Sleep(1500 * time.Millisecond)
	count := collector.count()
	if count < 2 {
		t.Errorf("expected at least 2 ticks with @every 500ms in 1.5s, got %d", count)
	}
}

func TestCronRestartSurvival(t *testing.T) {
	rt := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 3}),
	)

	// Create an actor that panics once then works fine.
	var mu sync.Mutex
	crashed := false
	ticks := make(chan actor.CronTickMessage, 64)

	handler := func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if tick, ok := msg.Payload.(actor.CronTickMessage); ok {
			mu.Lock()
			if !crashed {
				crashed = true
				mu.Unlock()
				return nil, fmt.Errorf("intentional crash")
			}
			mu.Unlock()
			select {
			case ticks <- tick:
			default:
			}
		}
		return state, nil
	}

	rt.CreateActor("worker", nil, handler)
	pid, _ := rt.IssuePID("node-1", "worker")

	time.Sleep(50 * time.Millisecond)

	ctx := context.Background()
	ack, err := rt.CronSubscribe(ctx, "", pid, "@every 500ms", "survive", "", 5*time.Second)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if ack.Result != actor.CronOutcomeSubscribeSuccess {
		t.Fatalf("expected subscribe_success, got %s", ack.Result)
	}

	// The first tick may cause a crash+restart. Subsequent ticks should arrive.
	select {
	case <-ticks:
		// Got a tick after restart — survival confirmed.
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for tick after restart")
	}
}
