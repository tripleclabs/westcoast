package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLifecycleSyncRemovesRegistryEntry(t *testing.T) {
	rt := actor.NewRuntime()
	ref, _ := rt.CreateActor("ls1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	_, _ = ref.RegisterName("life-graceful")
	if got := ref.Stop(); got != actor.StopStopped {
		t.Fatalf("stop=%s", got)
	}
	assertLookupMiss(t, rt.LookupName("life-graceful"))
}
