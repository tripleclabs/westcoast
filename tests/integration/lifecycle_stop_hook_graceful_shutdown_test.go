package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLifecycleStopHookRunsDuringGracefulShutdown(t *testing.T) {
	rt := actor.NewRuntime()
	stopped := make(chan struct{}, 1)
	ref, err := rt.CreateActor("stop-hook", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithStopHook(func(_ context.Context, _ string) error {
		stopped <- struct{}{}
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}

	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("stop hook did not execute")
	}
	assertLifecycleResult(t, ref.LifecycleOutcomes(), actor.LifecyclePhaseStop, actor.LifecycleStopSuccess)
}
