package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBatchProcessingResumesAfterRestart(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &countingBatchReceiver{}
	receiver.triggerFailure()
	ref, err := rt.CreateActor("batch-restart", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(8, receiver))
	if err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), 1); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send rejected: %s", ack.Result)
	}
	waitFor(t, time.Second, func() bool {
		for _, out := range ref.BatchOutcomes() {
			if out.Result == actor.BatchResultFailedHandler {
				return true
			}
		}
		return false
	}, "expected initial batch failure")

	for i := 0; i < 20; i++ {
		if ack := ref.Send(context.Background(), i+100); ack.Result != actor.SubmitAccepted {
			t.Fatalf("send %d rejected: %s", i, ack.Result)
		}
	}
	waitFor(t, time.Second, func() bool {
		sizes, _, processed := receiver.stats()
		if processed < 20 {
			return false
		}
		for _, n := range sizes {
			if n > 1 {
				return true
			}
		}
		return false
	}, "expected resumed batching with multi-message cycles")
}
