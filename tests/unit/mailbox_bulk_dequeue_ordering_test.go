package unit_test

import (
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestMailboxBulkDequeueOrderingAndBounds(t *testing.T) {
	mb := actor.NewMailbox(16)
	for i := 0; i < 6; i++ {
		res := mb.Enqueue(actor.Message{ID: uint64(i + 1), Payload: i, AcceptedAt: time.Now()})
		if res != actor.SubmitAccepted {
			t.Fatalf("enqueue %d failed: %s", i, res)
		}
	}
	batch := mb.DequeueBatch(4)
	if len(batch) != 4 {
		t.Fatalf("expected first batch size 4, got %d", len(batch))
	}
	for i := 0; i < 4; i++ {
		if got := batch[i].Payload.(int); got != i {
			t.Fatalf("out-of-order payload at %d: got %d", i, got)
		}
	}
	batch = mb.DequeueBatch(4)
	if len(batch) != 2 {
		t.Fatalf("expected second batch size 2, got %d", len(batch))
	}
	if batch[0].Payload.(int) != 4 || batch[1].Payload.(int) != 5 {
		t.Fatalf("unexpected second batch payloads: %+v", batch)
	}
	if more := mb.DequeueBatch(4); len(more) != 0 {
		t.Fatalf("expected empty batch, got %d", len(more))
	}
}
