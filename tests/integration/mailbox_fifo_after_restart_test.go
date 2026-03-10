package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type fifoMsg struct {
	N     int
	Panic bool
	Ch    chan []int
}

func fifoHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.([]int)
	s := msg.Payload.(fifoMsg)
	if s.Panic {
		panic("fifo panic")
	}
	if s.Ch != nil {
		out := append([]int(nil), cur...)
		s.Ch <- out
		return cur, nil
	}
	cur = append(cur, s.N)
	return cur, nil
}

func TestMailboxFIFOAfterRestart(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}))
	ref, err := rt.CreateActor("mb-fifo", []int{}, fifoHandler, actor.WithMailboxCapacity(64))
	if err != nil {
		t.Fatal(err)
	}
	_ = ref.Send(context.Background(), fifoMsg{N: 0, Panic: true})
	_ = ref.Send(context.Background(), fifoMsg{N: 1})
	_ = ref.Send(context.Background(), fifoMsg{N: 2})
	_ = ref.Send(context.Background(), fifoMsg{N: 3})

	ch := make(chan []int, 1)
	_ = ref.Send(context.Background(), fifoMsg{Ch: ch})
	select {
	case got := <-ch:
		want := []int{1, 2, 3}
		if len(got) != len(want) {
			t.Fatalf("len=%d want=%d (%v)", len(got), len(want), got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("got=%v want=%v", got, want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting fifo query")
	}
}
