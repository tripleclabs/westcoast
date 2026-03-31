package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestMonitor_ReceivesDownOnStop(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	// Create a watcher actor that collects DownMessages.
	downCh := make(chan actor.DownMessage, 4)
	watcherHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		if down, ok := msg.Payload.(actor.DownMessage); ok {
			downCh <- down
		}
		return state, nil
	}
	rt.CreateActor("watcher", nil, watcherHandler)
	watcherPID, _ := rt.IssuePID("node-1", "watcher")

	// Create a target actor.
	targetHandler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}
	rt.CreateActor("target", nil, targetHandler)
	targetPID, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	// Monitor target.
	ref := rt.Monitor(watcherPID, targetPID)
	_ = ref

	// Stop target.
	rt.Stop("target")

	// Watcher should receive a DownMessage.
	select {
	case down := <-downCh:
		if down.Target.ActorID != "target" {
			t.Errorf("target: got %s", down.Target.ActorID)
		}
		if down.Reason != "stopped" {
			t.Errorf("reason: got %s", down.Reason)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for DownMessage")
	}
}

func TestMonitor_Demonitor(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	downCh := make(chan actor.DownMessage, 4)
	rt.CreateActor("watcher", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if down, ok := msg.Payload.(actor.DownMessage); ok {
			downCh <- down
		}
		return state, nil
	})
	watcherPID, _ := rt.IssuePID("node-1", "watcher")

	rt.CreateActor("target", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	})
	targetPID, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	ref := rt.Monitor(watcherPID, targetPID)
	rt.Demonitor(ref)

	rt.Stop("target")

	// Should NOT receive a DownMessage.
	select {
	case <-downCh:
		t.Error("should not receive DownMessage after demonitor")
	case <-time.After(200 * time.Millisecond):
		// good
	}
}

func TestMonitor_MultipleWatchers(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	downCh1 := make(chan actor.DownMessage, 4)
	downCh2 := make(chan actor.DownMessage, 4)

	rt.CreateActor("w1", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if down, ok := msg.Payload.(actor.DownMessage); ok {
			downCh1 <- down
		}
		return state, nil
	})
	rt.CreateActor("w2", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if down, ok := msg.Payload.(actor.DownMessage); ok {
			downCh2 <- down
		}
		return state, nil
	})

	pid1, _ := rt.IssuePID("node-1", "w1")
	pid2, _ := rt.IssuePID("node-1", "w2")

	rt.CreateActor("target", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	})
	targetPID, _ := rt.IssuePID("node-1", "target")

	time.Sleep(50 * time.Millisecond)

	rt.Monitor(pid1, targetPID)
	rt.Monitor(pid2, targetPID)

	rt.Stop("target")

	// Both watchers should receive DownMessage.
	for _, ch := range []chan actor.DownMessage{downCh1, downCh2} {
		select {
		case down := <-ch:
			if down.Target.ActorID != "target" {
				t.Errorf("got %s", down.Target.ActorID)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	}
}

func TestMonitor_RefIsUnique(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	pid := actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1}
	target := actor.PID{Namespace: "node-1", ActorID: "b", Generation: 1}

	r1 := rt.Monitor(pid, target)
	r2 := rt.Monitor(pid, target)

	if r1.ID() == r2.ID() {
		t.Error("monitor refs should be unique")
	}
}
