package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

type supportedOnly struct{ Value int }

func (supportedOnly) SchemaVersion() string { return "v1" }

type unsupported struct{ Value int }

func TestRejectUnsupportedType(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("reject-unsupported", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("integration_test.supportedOnly", "v1", "default"); err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), unsupported{Value: 1})
	if ack.Result != actor.SubmitRejectedUnsupportedType {
		t.Fatalf("result=%s", ack.Result)
	}
	out, ok := rt.Outcome(ack.MessageID)
	if !ok {
		t.Fatal("missing outcome")
	}
	if out.Result != actor.ResultRejectedUnsupportedType {
		t.Fatalf("outcome=%s", out.Result)
	}
}
