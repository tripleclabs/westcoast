package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestContractSendMessageOutcomes(t *testing.T) {
	rt := actor.NewRuntime()

	ack := rt.Send(context.Background(), "missing", "x")
	if ack.Result != actor.SubmitRejectedFound {
		t.Fatalf("missing actor result = %s", ack.Result)
	}

	ref, err := rt.CreateActor("s", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}
	if ack := ref.Send(context.Background(), "hello"); ack.Result != actor.SubmitAccepted {
		t.Fatalf("accepted result = %s", ack.Result)
	}
	_ = ref.Stop()
	if ack := ref.Send(context.Background(), "after-stop"); ack.Result != actor.SubmitRejectedStop {
		t.Fatalf("after stop result = %s", ack.Result)
	}
}
