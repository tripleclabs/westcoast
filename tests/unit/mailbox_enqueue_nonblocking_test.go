package unit_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type block struct {
	Release <-chan struct{}
	Started chan<- struct{}
}

func blockingHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	if b, ok := msg.Payload.(block); ok {
		if b.Started != nil {
			b.Started <- struct{}{}
		}
		<-b.Release
	}
	return state, nil
}

func TestMailboxEnqueueNonBlocking(t *testing.T) {
	rt := actor.NewRuntime()
	release := make(chan struct{})
	startSignal := make(chan struct{}, 1)
	ref, err := rt.CreateActor("q", 0, blockingHandler, actor.WithMailboxCapacity(1))
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	if r := ref.Send(context.Background(), block{Release: release, Started: startSignal}).Result; r != actor.SubmitAccepted {
		t.Fatalf("first send result = %s", r)
	}
	select {
	case <-startSignal:
	case <-time.After(time.Second):
		t.Fatal("first message did not start processing")
	}
	if r := ref.Send(context.Background(), block{Release: release}).Result; r != actor.SubmitAccepted {
		t.Fatalf("second send result = %s", r)
	}

	startedAt := time.Now()
	res := ref.Send(context.Background(), block{Release: release}).Result
	elapsed := time.Since(startedAt)
	if res != actor.SubmitRejectedFull {
		t.Fatalf("third send result = %s, want rejected_full", res)
	}
	if elapsed > 50*time.Millisecond {
		t.Fatalf("enqueue blocked for %s", elapsed)
	}
	close(release)
}
