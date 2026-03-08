package integration_test

import (
	"context"
	"errors"
	"testing"

	"westcoast/src/actor"
)

func TestDirectStateMutationRejected(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("immutable", 1, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	if err := ref.SetState(99); !errors.Is(err, actor.ErrStateMutationForbidden) {
		t.Fatalf("expected ErrStateMutationForbidden, got %v", err)
	}
}
