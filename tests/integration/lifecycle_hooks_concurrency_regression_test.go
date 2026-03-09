package integration_test

import (
	"context"
	"sync"
	"testing"

	"westcoast/src/actor"
)

func TestLifecycleHooksConcurrentStartStopInterleavings(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("conc-life", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithStartHook(func(_ context.Context, _ string) error { return nil }), actor.WithStopHook(func(_ context.Context, _ string) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = ref.Send(context.Background(), i)
		}()
	}
	wg.Wait()
	_ = ref.Stop()
	if ref.Status() != actor.ActorStopped {
		t.Fatalf("status=%s", ref.Status())
	}
}
