package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestEdgeCases(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("edge", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithMailboxCapacity(1))
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	if _, err := rt.CreateActor("edge", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil }); err == nil {
		t.Fatal("expected duplicate actor id error")
	}

	_ = ref.Stop()
	if ack := ref.Send(context.Background(), "after-stop"); ack.Result != actor.SubmitRejectedStop {
		t.Fatalf("expected rejected_stopped, got %s", ack.Result)
	}
}
