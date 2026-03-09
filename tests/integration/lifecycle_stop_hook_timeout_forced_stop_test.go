package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleStopHookTimeoutForcesStop(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("stop-timeout", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithStopHook(func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}), actor.WithStopHookTimeout(20*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	if took := time.Since(started); took > 250*time.Millisecond {
		t.Fatalf("stop took too long: %s", took)
	}
	if ref.Status() != actor.ActorStopped {
		t.Fatalf("status=%s", ref.Status())
	}
	outs := ref.LifecycleOutcomes()
	assertLifecycleResult(t, outs, actor.LifecyclePhaseStop, actor.LifecycleStopFailed)
	last := outs[len(outs)-1]
	if last.ErrorCode != "timeout" {
		t.Fatalf("expected timeout code, got %q", last.ErrorCode)
	}
}
