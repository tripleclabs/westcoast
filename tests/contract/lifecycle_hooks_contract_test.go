package contract_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleHooksOutcomeContract(t *testing.T) {
	vals := []actor.LifecycleHookResult{
		actor.LifecycleStartSuccess,
		actor.LifecycleStartFailed,
		actor.LifecycleStopSuccess,
		actor.LifecycleStopFailed,
	}
	for _, v := range vals {
		if string(v) == "" {
			t.Fatal("empty lifecycle result")
		}
	}
}

func TestStartHookExecutionContract(t *testing.T) {
	rt := actor.NewRuntime()
	startDone := make(chan struct{}, 1)
	ref, err := rt.CreateActor("lc-start", 0, noop,
		actor.WithStartHook(func(_ context.Context, _ string) error {
			startDone <- struct{}{}
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	waitForLifecycleContractStatus(t, ref, actor.ActorRunning)
	select {
	case <-startDone:
	default:
		t.Fatal("start hook did not run")
	}
}

func TestStopHookExecutionContract(t *testing.T) {
	rt := actor.NewRuntime()
	stopDone := make(chan struct{}, 1)
	ref, err := rt.CreateActor("lc-stop", 0, noop,
		actor.WithStopHook(func(_ context.Context, _ string) error {
			stopDone <- struct{}{}
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	select {
	case <-stopDone:
	default:
		t.Fatal("stop hook did not run")
	}
}

func TestStopHookFailureStillStopsContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("lc-stop-fail", 0, noop,
		actor.WithStopHook(func(_ context.Context, _ string) error {
			return errors.New("cleanup failed")
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	waitForLifecycleContractStatus(t, ref, actor.ActorStopped)
	outs := ref.LifecycleOutcomes()
	if len(outs) == 0 {
		t.Fatal("expected lifecycle outcomes")
	}
	found := false
	for _, out := range outs {
		if out.Phase == actor.LifecyclePhaseStop && out.Result == actor.LifecycleStopFailed {
			if out.ErrorCode == "" {
				t.Fatal("expected stop failure error code")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("missing stop_failed lifecycle outcome")
	}
}

func waitForLifecycleContractStatus(t *testing.T, ref *actor.ActorRef, want actor.ActorStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ref.Status() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status=%s want=%s", ref.Status(), want)
}
