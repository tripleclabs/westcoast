package integration_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleStartHookFailurePreventsRunning(t *testing.T) {
	rt := actor.NewRuntime()
	var processed atomic.Int32
	ref, err := rt.CreateActor("start-fail", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		processed.Add(1)
		return state, nil
	}, actor.WithStartHook(func(_ context.Context, _ string) error {
		return errors.New("boom")
	}))
	if err != nil {
		t.Fatal(err)
	}

	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorStopped }, "expected stopped after start hook failure")
	ack := ref.Send(context.Background(), "later")
	if ack.Result != actor.SubmitRejectedStop {
		t.Fatalf("send result=%s", ack.Result)
	}
	if processed.Load() != 0 {
		t.Fatal("message should not be processed on start hook failure")
	}
	assertLifecycleResult(t, ref.LifecycleOutcomes(), actor.LifecyclePhaseStart, actor.LifecycleStartFailed)
}
