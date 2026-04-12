package cluster

import (
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestSendAfterName_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	rt.RegisterName("target", "my-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	rt.SendAfterName("my-service", "delayed-by-name", 100*time.Millisecond)

	// Should not arrive immediately.
	time.Sleep(50 * time.Millisecond)
	if collector.count() != 0 {
		t.Error("message arrived too early")
	}

	msg := collector.waitMessage(t, 200*time.Millisecond)
	if msg != "delayed-by-name" {
		t.Errorf("got %v", msg)
	}
}

func TestSendAfterName_Cancel(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	rt.RegisterName("target", "my-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendAfterName("my-service", "should-not-arrive", 200*time.Millisecond)
	ref.Cancel()

	time.Sleep(300 * time.Millisecond)
	if collector.count() != 0 {
		t.Error("cancelled timer should not deliver")
	}
}

func TestSendAfterName_NotRegistered(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	// No actor registered with this name — should not panic, just drop.
	ref := rt.SendAfterName("nonexistent", "dropped", 50*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	ref.Cancel()
}

func TestSendIntervalName_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	rt.RegisterName("target", "my-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendIntervalName("my-service", "tick", 50*time.Millisecond)

	time.Sleep(280 * time.Millisecond)
	ref.Cancel()

	count := collector.count()
	if count < 3 || count > 7 {
		t.Errorf("expected ~5 ticks, got %d", count)
	}
}

func TestSendIntervalName_Cancel(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	rt.RegisterName("target", "my-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendIntervalName("my-service", "tick", 50*time.Millisecond)
	time.Sleep(130 * time.Millisecond)
	ref.Cancel()

	countAtCancel := collector.count()
	time.Sleep(200 * time.Millisecond)
	countAfter := collector.count()

	if countAfter != countAtCancel {
		t.Errorf("messages arrived after cancel: before=%d, after=%d", countAtCancel, countAfter)
	}
}

func TestSendAfterName_ResolvesAtDeliveryTime(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector1 := newCollectingHandler()
	collector2 := newCollectingHandler()
	rt.CreateActor("actor-1", nil, collector1.Handle)
	rt.CreateActor("actor-2", nil, collector2.Handle)

	// Register name to actor-1 initially.
	rt.RegisterName("actor-1", "the-service", "node-1")

	time.Sleep(50 * time.Millisecond)

	// Schedule delivery in 200ms.
	rt.SendAfterName("the-service", "who-gets-it", 200*time.Millisecond)

	// Re-register name to actor-2 before delivery.
	time.Sleep(50 * time.Millisecond)
	rt.UnregisterName("the-service")
	rt.RegisterName("actor-2", "the-service", "node-1")

	// actor-2 should receive it (resolved at delivery time).
	msg := collector2.waitMessage(t, 300*time.Millisecond)
	if msg != "who-gets-it" {
		t.Errorf("got %v", msg)
	}

	// actor-1 should NOT have received it.
	if collector1.count() != 0 {
		t.Error("actor-1 should not have received the message")
	}
}
