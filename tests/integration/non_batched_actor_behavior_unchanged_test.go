package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestNonBatchedActorBehaviorUnchanged(t *testing.T) {
	rt := actor.NewRuntime()
	var seen atomic.Int64
	ref, err := rt.CreateActor("non-batch", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		seen.Add(1)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		ack := ref.Send(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("send %d rejected: %s", i, ack.Result)
		}
	}
	waitFor(t, time.Second, func() bool { return seen.Load() >= 10 }, "expected all non-batch messages processed")
	if outs := ref.BatchOutcomes(); len(outs) != 0 {
		t.Fatalf("expected no batch outcomes for non-batching actor, got %d", len(outs))
	}

	_ = time.Second
}
