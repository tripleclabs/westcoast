package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type inc struct{ N int }
type get struct{ Ch chan int }

func counterHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.(int)
	switch p := msg.Payload.(type) {
	case inc:
		return cur + p.N, nil
	case get:
		p.Ch <- cur
		return cur, nil
	default:
		return cur, nil
	}
}

func TestActorStateIsolation(t *testing.T) {
	rt := actor.NewRuntime()
	a1, err := rt.CreateActor("a1", 0, counterHandler)
	if err != nil {
		t.Fatalf("create a1: %v", err)
	}
	a2, err := rt.CreateActor("a2", 0, counterHandler)
	if err != nil {
		t.Fatalf("create a2: %v", err)
	}

	if ack := a1.Send(context.Background(), inc{N: 5}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("unexpected result: %s", ack.Result)
	}

	ch1 := make(chan int, 1)
	ch2 := make(chan int, 1)
	a1.Send(context.Background(), get{Ch: ch1})
	a2.Send(context.Background(), get{Ch: ch2})

	select {
	case got := <-ch1:
		if got != 5 {
			t.Fatalf("actor1 = %d, want 5", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting actor1")
	}

	select {
	case got := <-ch2:
		if got != 0 {
			t.Fatalf("actor2 = %d, want 0", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting actor2")
	}
}
