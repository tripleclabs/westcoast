package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func panicHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.(int)
	switch p := msg.Payload.(type) {
	case panicMsg:
		panic("boom")
	case inc:
		return cur + p.N, nil
	case counterQuery:
		p.Ch <- cur
		return cur, nil
	default:
		return cur, nil
	}
}

func TestActorFailureIsolation(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	a, err := rt.CreateActor("a", 0, panicHandler)
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	b, err := rt.CreateActor("b", 0, panicHandler)
	if err != nil {
		t.Fatalf("create b: %v", err)
	}

	panicAck := a.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool {
		out, ok := rt.Outcome(panicAck.MessageID)
		return ok && out.Result == actor.ResultFailed
	}, "expected failed outcome for panic message")

	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventActorFailed && e.ActorID == "a" && e.MessageID == panicAck.MessageID {
				return true
			}
		}
		return false
	}, "expected actor_failed event for panic message")

	if ack := b.Send(context.Background(), inc{N: 1}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("b send result = %s", ack.Result)
	}
	ch := make(chan int, 1)
	_ = b.Send(context.Background(), counterQuery{Ch: ch})
	select {
	case got := <-ch:
		if got != 1 {
			t.Fatalf("b state = %d, want 1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting b")
	}
}
