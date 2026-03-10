package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBatchReceiveRespectsConfiguredBatchLimit(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &countingBatchReceiver{}
	ref, err := rt.CreateActor("batch-limit", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(3, receiver))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 10; i++ {
		ack := ref.Send(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("send %d rejected: %s", i, ack.Result)
		}
	}
	waitFor(t, time.Second, func() bool {
		_, _, processed := receiver.stats()
		return processed >= 10
	}, "expected processing of all messages")

	sizes, _, _ := receiver.stats()
	for _, n := range sizes {
		if n <= 0 || n > 3 {
			t.Fatalf("batch size exceeded limit: %d", n)
		}
	}
}
