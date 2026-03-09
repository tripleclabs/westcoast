package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleStopHookFailureStillStopsActor(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("stop-fail", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithStopHook(func(_ context.Context, _ string) error {
		return errors.New("cleanup")
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorStopped }, "expected stopped")
	assertLifecycleResult(t, ref.LifecycleOutcomes(), actor.LifecyclePhaseStop, actor.LifecycleStopFailed)
}
