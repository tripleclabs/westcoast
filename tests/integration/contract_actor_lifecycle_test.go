package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestContractActorLifecycle(t *testing.T) {
	rt := actor.NewRuntime()
	_, err := rt.CreateActor("lifecycle", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}
	if _, err := rt.CreateActor("lifecycle", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}); err == nil {
		t.Fatal("expected duplicate actor id error")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if rt.Status("lifecycle") == actor.ActorRunning {
			break
		}
		time.Sleep(time.Millisecond * 5)
	}
	if rt.Status("lifecycle") != actor.ActorRunning {
		t.Fatalf("status = %s, want running", rt.Status("lifecycle"))
	}

	if got := rt.Stop("lifecycle"); got != actor.StopStopped {
		t.Fatalf("stop = %s, want stopped", got)
	}
	if got := rt.Stop("lifecycle"); got != actor.StopAlready {
		t.Fatalf("stop second = %s, want already_stopped", got)
	}
}
