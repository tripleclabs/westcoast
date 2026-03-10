package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLifecycleOutcomesObservable(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	ref, err := rt.CreateActor("obs", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	},
		actor.WithStartHook(func(_ context.Context, _ string) error { return nil }),
		actor.WithStopHook(func(_ context.Context, _ string) error { return errors.New("x") }),
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop result=%s", got)
	}
	waitFor(t, time.Second, func() bool {
		evs := emitter.Events()
		foundStart := false
		foundStop := false
		for _, e := range evs {
			if e.Type != actor.EventLifecycleHook {
				continue
			}
			if e.LifecyclePhase == string(actor.LifecyclePhaseStart) && e.Result == string(actor.LifecycleStartSuccess) {
				foundStart = true
			}
			if e.LifecyclePhase == string(actor.LifecyclePhaseStop) && e.Result == string(actor.LifecycleStopFailed) {
				foundStop = true
			}
		}
		return foundStart && foundStop
	}, "expected lifecycle hook events")
}
