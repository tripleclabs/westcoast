package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type seqMsg struct {
	N      int
	Panic  bool
	DoneCh chan int
}

func seqHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.([]int)
	s := msg.Payload.(seqMsg)
	if s.Panic {
		panic("seq panic")
	}
	cur = append(cur, s.N)
	if s.DoneCh != nil {
		s.DoneCh <- len(cur)
	}
	return cur, nil
}

func TestMailboxPreservationAcrossRestart(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}))
	ref, err := rt.CreateActor("mb-preserve", []int{}, seqHandler, actor.WithMailboxCapacity(64))
	if err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), seqMsg{N: 1, Panic: true}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("panic send=%s", ack.Result)
	}
	if ack := ref.Send(context.Background(), seqMsg{N: 2}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send2=%s", ack.Result)
	}
	done := make(chan int, 1)
	if ack := ref.Send(context.Background(), seqMsg{N: 3, DoneCh: done}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send3=%s", ack.Result)
	}
	select {
	case got := <-done:
		if got != 2 {
			t.Fatalf("processed count=%d want=2", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting preserved messages")
	}
}
