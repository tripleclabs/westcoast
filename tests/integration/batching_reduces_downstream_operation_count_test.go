package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBatchingReducesDownstreamOperationCount(t *testing.T) {
	rt := actor.NewRuntime()
	var plainCalls atomic.Int64
	plain, err := rt.CreateActor("plain-io", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		plainCalls.Add(1)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	receiver := &countingBatchReceiver{}
	batched, err := rt.CreateActor("batched-io", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(25, receiver))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 200; i++ {
		if ack := plain.Send(context.Background(), i); ack.Result != actor.SubmitAccepted {
			t.Fatalf("plain send rejected: %s", ack.Result)
		}
		if ack := batched.Send(context.Background(), i); ack.Result != actor.SubmitAccepted {
			t.Fatalf("batched send rejected: %s", ack.Result)
		}
	}
	waitFor(t, time.Second, func() bool { return plainCalls.Load() >= 200 }, "expected plain processing complete")
	waitFor(t, time.Second, func() bool {
		_, _, processed := receiver.stats()
		return processed >= 200
	}, "expected batched processing complete")

	_, bulkOps, _ := receiver.stats()
	if bulkOps >= int(plainCalls.Load()) {
		t.Fatalf("expected fewer batched operations than plain calls: bulkOps=%d plain=%d", bulkOps, plainCalls.Load())
	}
}
