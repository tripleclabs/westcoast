package contract_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func noop(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil }

func TestPIDClosedOutcomeSet(t *testing.T) {
	vals := []actor.PIDDeliveryOutcome{actor.PIDDelivered, actor.PIDUnresolved, actor.PIDRejectedStopped, actor.PIDRejectedStaleGeneration, actor.PIDRejectedNotFound}
	for _, v := range vals {
		s := string(v)
		if s == "" {
			t.Fatal("empty outcome")
		}
	}
}

func TestPIDIssueResolveContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("c1", 0, noop)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := ref.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := rt.ResolvePID(pid)
	if !ok || entry.CurrentGeneration != pid.Generation {
		t.Fatalf("bad resolve: %+v ok=%v", entry, ok)
	}
}
