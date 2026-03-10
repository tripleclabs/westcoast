package unit_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func waitNotify(t *testing.T, ch <-chan struct{}, d time.Duration) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(d):
		t.Fatal("timeout waiting mailbox notify")
	}
}

func TestMailboxCloseRejectsFurtherEnqueue(t *testing.T) {
	mb := actor.NewMailbox(8)
	if got := mb.Enqueue(actor.Message{ID: 1}); got != actor.SubmitAccepted {
		t.Fatalf("enqueue before close=%s", got)
	}
	mb.Close()
	if got := mb.Enqueue(actor.Message{ID: 2}); got != actor.SubmitRejectedStop {
		t.Fatalf("enqueue after close=%s", got)
	}
}

func TestMailboxFIFOAndCompactionReuse(t *testing.T) {
	const n = 2000
	mb := actor.NewMailbox(n + 8)

	for i := 0; i < n; i++ {
		if got := mb.Enqueue(actor.Message{ID: uint64(i + 1)}); got != actor.SubmitAccepted {
			t.Fatalf("enqueue %d=%s", i, got)
		}
	}
	if depth := mb.Depth(); depth != n {
		t.Fatalf("depth=%d want=%d", depth, n)
	}

	waitNotify(t, mb.Notify(), time.Second)
	for i := 0; i < n; i++ {
		msg, ok := mb.Dequeue()
		if !ok {
			t.Fatalf("dequeue %d failed", i)
		}
		want := uint64(i + 1)
		if msg.ID != want {
			t.Fatalf("msg[%d].ID=%d want=%d", i, msg.ID, want)
		}
	}
	if _, ok := mb.Dequeue(); ok {
		t.Fatal("expected empty queue")
	}
	if depth := mb.Depth(); depth != 0 {
		t.Fatalf("depth after drain=%d", depth)
	}

	// Ensure queue remains reusable after compaction.
	if got := mb.Enqueue(actor.Message{ID: 9001}); got != actor.SubmitAccepted {
		t.Fatalf("enqueue after compaction=%s", got)
	}
	waitNotify(t, mb.Notify(), time.Second)
	msg, ok := mb.Dequeue()
	if !ok || msg.ID != 9001 {
		t.Fatalf("post-compaction dequeue ok=%v id=%d", ok, msg.ID)
	}
}

func TestMailboxConcurrentProducerConsumerNoLoss(t *testing.T) {
	const (
		producers          = 8
		perProducer        = 2000
		total              = producers * perProducer
		mailboxMaxCapacity = 256
	)

	mb := actor.NewMailbox(mailboxMaxCapacity)
	var delivered atomic.Int64
	seen := make([]atomic.Bool, total+1)
	done := make(chan struct{})

	// Single consumer mimics actor runtime drain behavior.
	go func() {
		defer close(done)
		for delivered.Load() < total {
			<-mb.Notify()
			for {
				msg, ok := mb.Dequeue()
				if !ok {
					break
				}
				id := int(msg.ID)
				if id <= 0 || id > total {
					panic("invalid message id")
				}
				if seen[id].Swap(true) {
					panic("duplicate message id")
				}
				delivered.Add(1)
			}
		}
	}()

	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			base := p * perProducer
			for i := 1; i <= perProducer; i++ {
				id := uint64(base + i)
				for {
					switch mb.Enqueue(actor.Message{ID: id}) {
					case actor.SubmitAccepted:
						goto enqueued
					case actor.SubmitRejectedFull:
						time.Sleep(time.Microsecond)
					default:
						t.Errorf("unexpected enqueue result for id=%d", id)
						return
					}
				}
			enqueued:
			}
		}()
	}

	wg.Wait()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting consumer completion")
	}

	if got := int(delivered.Load()); got != total {
		t.Fatalf("delivered=%d want=%d", got, total)
	}
	for i := 1; i <= total; i++ {
		if !seen[i].Load() {
			t.Fatalf("missing message id=%d", i)
		}
	}
}
