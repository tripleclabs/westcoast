package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRejectNilPayload(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("reject-nil", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), nil)
	if ack.Result != actor.SubmitRejectedNilPayload {
		t.Fatalf("result=%s", ack.Result)
	}
	out, ok := rt.Outcome(ack.MessageID)
	if !ok {
		t.Fatal("missing outcome")
	}
	if out.Result != actor.ResultRejectedNilPayload {
		t.Fatalf("outcome=%s", out.Result)
	}
}
