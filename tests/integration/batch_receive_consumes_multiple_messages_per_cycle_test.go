package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

var errBatchReceiverFail = errors.New("batch_receiver_fail")

func TestBatchReceiveConsumesMultipleMessagesPerCycle(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &countingBatchReceiver{}
	ref, err := rt.CreateActor("batch-cycle", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(50, receiver))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		ack := ref.Send(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("send %d rejected: %s", i, ack.Result)
		}
	}

	waitFor(t, time.Second, func() bool {
		sizes, _, processed := receiver.stats()
		if processed < 50 {
			return false
		}
		for _, n := range sizes {
			if n > 1 {
				return true
			}
		}
		return false
	}, "expected at least one multi-message batch cycle")
}
