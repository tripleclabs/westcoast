package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBatchingEnablesSingleBulkDownstreamOperation(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &countingBatchReceiver{}
	ref, err := rt.CreateActor("batch-io-single", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(100, receiver))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 40; i++ {
		ack := ref.Send(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("send %d rejected: %s", i, ack.Result)
		}
	}
	waitFor(t, time.Second, func() bool {
		_, _, processed := receiver.stats()
		return processed >= 40
	}, "expected all messages processed")

	sizes, bulkOps, processed := receiver.stats()
	if processed != 40 {
		t.Fatalf("processed=%d, want 40", processed)
	}
	if bulkOps >= 40 {
		t.Fatalf("expected batched bulk ops < messages, got bulkOps=%d", bulkOps)
	}
	multi := false
	for _, n := range sizes {
		if n > 1 {
			multi = true
			break
		}
	}
	if !multi {
		t.Fatalf("expected at least one grouped operation, sizes=%v", sizes)
	}
}
