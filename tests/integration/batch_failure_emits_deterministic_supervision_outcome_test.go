package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBatchFailureEmitsDeterministicSupervisionOutcome(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &countingBatchReceiver{}
	receiver.triggerFailure()
	ref, err := rt.CreateActor("batch-failure", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(10, receiver))
	if err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), 1); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send rejected: %s", ack.Result)
	}
	waitFor(t, time.Second, func() bool {
		outs := ref.BatchOutcomes()
		for _, out := range outs {
			if out.Result == actor.BatchResultFailedHandler {
				return true
			}
		}
		return false
	}, "expected batch failure outcome")
}
