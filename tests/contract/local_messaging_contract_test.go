package contract_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type contractPayload struct {
	Value   int
	Version string
}

func (m contractPayload) SchemaVersion() string {
	if m.Version == "" {
		return "v1"
	}
	return m.Version
}

func TestLocalMessagingTerminalOutcomeSet(t *testing.T) {
	vals := []string{
		string(actor.ResultDelivered),
		string(actor.ResultRejectedUnsupportedType),
		string(actor.ResultRejectedNilPayload),
		string(actor.ResultRejectedVersionMismatch),
	}
	for _, v := range vals {
		if v == "" {
			t.Fatal("empty terminal outcome")
		}
	}
}

func TestLocalMessageMetadataPreservation(t *testing.T) {
	got := make(chan actor.Message, 1)
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("meta", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if _, ok := msg.Payload.(contractPayload); ok {
			got <- msg
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("contract_test.contractPayload", "v1", "default"); err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), contractPayload{Value: 1, Version: "v1"}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result = %s", ack.Result)
	}
	select {
	case msg := <-got:
		if msg.TypeName != "contract_test.contractPayload" {
			t.Fatalf("type_name=%q", msg.TypeName)
		}
		if msg.SchemaVersion != "v1" {
			t.Fatalf("schema_version=%q", msg.SchemaVersion)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestLocalRoutingDeterminismByTypeAndVersion(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("det", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := ref.RegisterTypeRoute("contract_test.contractPayload", "v1", "default"); err != nil {
		t.Fatal(err)
	}

	accepted := ref.Send(context.Background(), contractPayload{Value: 1, Version: "v1"})
	if accepted.Result != actor.SubmitAccepted {
		t.Fatalf("accepted result=%s", accepted.Result)
	}
	rejected := ref.Send(context.Background(), contractPayload{Value: 1, Version: "v2"})
	if rejected.Result != actor.SubmitRejectedVersionMismatch {
		t.Fatalf("result=%s want=%s", rejected.Result, actor.SubmitRejectedVersionMismatch)
	}
}
