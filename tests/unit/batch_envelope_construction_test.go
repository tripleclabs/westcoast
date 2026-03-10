package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type envelopeCaptureReceiver struct {
	sizes chan int
}

func (r envelopeCaptureReceiver) BatchReceive(_ context.Context, state any, payloads []any) (any, error) {
	r.sizes <- len(payloads)
	return state, nil
}

func TestBatchEnvelopeConstructionMetadata(t *testing.T) {
	em := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(em))
	receiver := envelopeCaptureReceiver{sizes: make(chan int, 8)}
	ref, err := rt.CreateActor("batch-envelope", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(5, receiver))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 7; i++ {
		if ack := ref.Send(context.Background(), i); ack.Result != actor.SubmitAccepted {
			t.Fatalf("send rejected: %s", ack.Result)
		}
	}
	deadline := time.Now().Add(time.Second)
	total := 0
	for total < 7 && time.Now().Before(deadline) {
		select {
		case n := <-receiver.sizes:
			total += n
		case <-time.After(10 * time.Millisecond):
		}
	}
	if total != 7 {
		t.Fatalf("expected 7 payloads processed, got %d", total)
	}
	events := em.Events()
	found := false
	for _, e := range events {
		if e.Type == actor.EventBatchLifecycle {
			found = true
			if e.BatchSize <= 0 || e.BatchSize > 5 {
				t.Fatalf("invalid event batch size: %+v", e)
			}
		}
	}
	if !found {
		t.Fatal("expected batch lifecycle events")
	}
}
