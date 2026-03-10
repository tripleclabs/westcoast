package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

type edgePayload struct {
	Version string
}

func (p edgePayload) SchemaVersion() string { return p.Version }

func TestVersionMismatchPreferredOverFallback(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("edge", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("integration_test.edgePayload", "v1", "exact"); err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterFallbackRoute("fallback"); err != nil {
		t.Fatal(err)
	}

	ack := ref.Send(context.Background(), edgePayload{Version: "v2"})
	if ack.Result != actor.SubmitRejectedVersionMismatch {
		t.Fatalf("unexpected result=%s", ack.Result)
	}
}
