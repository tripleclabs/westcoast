package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLogicalActorIDAddressing(t *testing.T) {
	rt := actor.NewRuntime()
	created, err := rt.CreateActor("logical-id-1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	ref, err := rt.ActorRef("logical-id-1")
	if err != nil {
		t.Fatalf("actor ref: %v", err)
	}
	if ref.ID() != created.ID() {
		t.Fatalf("id mismatch: %s != %s", ref.ID(), created.ID())
	}

	if _, err := rt.ActorRef("missing"); err == nil {
		t.Fatal("expected not found")
	}
}
