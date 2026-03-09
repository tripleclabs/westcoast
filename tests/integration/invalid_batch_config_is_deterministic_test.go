package integration_test

import (
	"context"
	"errors"
	"testing"

	"westcoast/src/actor"
)

func TestInvalidBatchConfigIsDeterministic(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("batch-invalid-config", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	err = ref.ConfigureBatching(0, &countingBatchReceiver{})
	if !errors.Is(err, actor.ErrBatchConfigInvalid) {
		t.Fatalf("expected ErrBatchConfigInvalid, got %v", err)
	}
}
