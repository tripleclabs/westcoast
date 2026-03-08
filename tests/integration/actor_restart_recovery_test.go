package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestActorRestartRecovery(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(
		actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}),
		actor.WithEmitter(emitter),
	)
	ref, err := rt.CreateActor("r", 0, panicHandler)
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	_ = ref.Send(context.Background(), inc{N: 5})
	panicAck := ref.Send(context.Background(), panicMsg{})

	waitFor(t, time.Second, func() bool {
		_, ok := rt.Outcome(panicAck.MessageID)
		return ok
	}, "expected failure outcome to be recorded")

	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventActorRestarted && e.ActorID == "r" && e.MessageID == panicAck.MessageID {
				return true
			}
		}
		return false
	}, "expected actor_restarted event for panic message")

	waitFor(t, time.Second, func() bool {
		return ref.Status() == actor.ActorRunning
	}, "actor did not recover to running state")
	if ref.Status() != actor.ActorRunning {
		t.Fatalf("status = %s, want running", ref.Status())
	}

	ch := make(chan int, 1)
	_ = ref.Send(context.Background(), counterQuery{Ch: ch})
	select {
	case got := <-ch:
		if got != 0 {
			t.Fatalf("restarted state = %d, want 0", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting state")
	}
}
