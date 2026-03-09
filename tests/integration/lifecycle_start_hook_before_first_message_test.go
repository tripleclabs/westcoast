package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleStartHookBlocksFirstMessageUntilInitialized(t *testing.T) {
	rt := actor.NewRuntime()
	var initialized atomic.Bool
	seenInit := make(chan bool, 1)

	ref, err := rt.CreateActor("start-before", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		seenInit <- initialized.Load()
		return state, nil
	}, actor.WithStartHook(func(_ context.Context, _ string) error {
		time.Sleep(30 * time.Millisecond)
		initialized.Store(true)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	ack := ref.Send(context.Background(), "go")
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}

	select {
	case got := <-seenInit:
		if !got {
			t.Fatal("message processed before start hook initialization")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for handler")
	}

	assertLifecycleResult(t, ref.LifecycleOutcomes(), actor.LifecyclePhaseStart, actor.LifecycleStartSuccess)
}
