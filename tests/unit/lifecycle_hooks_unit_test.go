package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLifecycleHooksOptionalByDefault(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("u-life", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ref.Status() == actor.ActorRunning {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ref.Status() != actor.ActorRunning {
		t.Fatalf("status=%s", ref.Status())
	}
	if len(ref.LifecycleOutcomes()) != 0 {
		t.Fatal("did not expect lifecycle outcomes without hooks")
	}
}
