package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestSendAfter_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	rt.SendAfter(pid, "delayed-msg", 100*time.Millisecond)

	// Should not arrive immediately.
	time.Sleep(50 * time.Millisecond)
	if collector.count() != 0 {
		t.Error("message arrived too early")
	}

	// Should arrive after delay.
	msg := collector.waitMessage(t, 200*time.Millisecond)
	if msg != "delayed-msg" {
		t.Errorf("got %v", msg)
	}
}

func TestSendAfter_Cancel(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendAfter(pid, "should-not-arrive", 200*time.Millisecond)
	ref.Cancel()

	time.Sleep(300 * time.Millisecond)
	if collector.count() != 0 {
		t.Error("cancelled timer should not deliver")
	}
}

func TestSendInterval_Local(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendInterval(pid, "tick", 50*time.Millisecond)

	// Wait for several ticks.
	time.Sleep(280 * time.Millisecond)
	ref.Cancel()

	count := collector.count()
	// Should have received 4-5 ticks in 280ms at 50ms intervals.
	if count < 3 || count > 7 {
		t.Errorf("expected ~5 ticks, got %d", count)
	}
}

func TestSendInterval_Cancel(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	collector := newCollectingHandler()
	rt.CreateActor("target", nil, collector.Handle)
	pid, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	ref := rt.SendInterval(pid, "tick", 50*time.Millisecond)
	time.Sleep(130 * time.Millisecond)
	ref.Cancel()

	countAtCancel := collector.count()
	time.Sleep(200 * time.Millisecond)
	countAfter := collector.count()

	if countAfter != countAtCancel {
		t.Errorf("messages arrived after cancel: before=%d, after=%d", countAtCancel, countAfter)
	}
}

func TestSendAfter_CrossNode(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register("")

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

	remoteSender := NewRemoteSender(c1, codec, nil)
	rt1 := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithRemoteSend(remoteSender.Send),
	)

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	collector := newCollectingHandler()
	rt2.CreateActor("target", nil, collector.Handle)
	remotePID, _ := rt2.IssuePID("node-2", "target")

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

	// Timer on node-1 sends to actor on node-2 after delay.
	rt1.SendAfter(remotePID, "delayed-cross-node", 100*time.Millisecond)

	msg := collector.waitMessage(t, 500*time.Millisecond)
	if msg != "delayed-cross-node" {
		t.Errorf("got %v", msg)
	}
}

// collectingHandler helper has count() method needed here.
func (h *collectingHandler) count() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.messages)
}
