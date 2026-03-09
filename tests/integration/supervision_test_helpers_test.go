package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func assertActorLivenessAfterPeerPanic(t *testing.T, rt *actor.Runtime, healthy *actor.ActorRef, panicking *actor.ActorRef) {
	t.Helper()
	panicAck := panicking.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool {
		out, ok := rt.Outcome(panicAck.MessageID)
		return ok && out.Result == actor.ResultFailed
	}, "expected failed outcome for panicking actor")

	if ack := healthy.Send(context.Background(), inc{N: 1}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("healthy actor send result=%s", ack.Result)
	}
	ch := make(chan int, 1)
	_ = healthy.Send(context.Background(), counterQuery{Ch: ch})
	select {
	case got := <-ch:
		if got != 1 {
			t.Fatalf("healthy actor state=%d want=1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for healthy actor query")
	}
}
