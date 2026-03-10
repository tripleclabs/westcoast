package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBatchOutcomeStoreQuerying(t *testing.T) {
	rt := actor.NewRuntime()
	receiver := &envelopeCaptureReceiver{sizes: make(chan int, 16)}
	ref, err := rt.CreateActor("batch-outcome-store", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(4, receiver))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		if ack := ref.Send(context.Background(), i); ack.Result != actor.SubmitAccepted {
			t.Fatalf("send rejected: %s", ack.Result)
		}
	}
	deadline := time.Now().Add(time.Second)
	total := 0
	for total < 12 && time.Now().Before(deadline) {
		select {
		case n := <-receiver.sizes:
			total += n
		case <-time.After(10 * time.Millisecond):
		}
	}
	outs := ref.BatchOutcomes()
	if len(outs) == 0 {
		t.Fatal("expected batch outcomes")
	}
	counted := 0
	for _, out := range outs {
		if out.ActorID != "batch-outcome-store" {
			t.Fatalf("unexpected actor id in outcome: %+v", out)
		}
		counted += out.BatchSize
	}
	if counted < 12 {
		t.Fatalf("expected at least 12 message slots in outcomes, got %d", counted)
	}
}
