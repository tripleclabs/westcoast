package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestCleanStateRestart(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}))
	ref, err := rt.CreateActor("clean", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), inc{N: 5}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("inc ack=%s", ack.Result)
	}
	panicAck := ref.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool {
		out, ok := rt.Outcome(panicAck.MessageID)
		return ok && out.Result == actor.ResultFailed
	}, "expected panic outcome")

	ch := make(chan int, 1)
	_ = ref.Send(context.Background(), counterQuery{Ch: ch})
	select {
	case got := <-ch:
		if got != 0 {
			t.Fatalf("state=%d want=0", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting query")
	}
}
