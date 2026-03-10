package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLifecycleStartHookRunsOnRestart(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}))
	var starts atomic.Int32
	ref, err := rt.CreateActor("start-restart", 0, panicHandler,
		actor.WithStartHook(func(_ context.Context, _ string) error {
			starts.Add(1)
			return nil
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorRunning }, "expected running")
	if starts.Load() < 1 {
		t.Fatal("expected initial start hook")
	}
	_ = ref.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool { return starts.Load() >= 2 }, "expected start hook rerun on restart")
}
