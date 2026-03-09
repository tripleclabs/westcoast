package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type localPayload struct{ Value int }

func (localPayload) SchemaVersion() string { return "v1" }

func TestNativeLocalDeliveryPreservesMetadata(t *testing.T) {
	got := make(chan actor.Message, 1)
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	ref, err := rt.CreateActor("native-delivery", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if _, ok := msg.Payload.(localPayload); ok {
			got <- msg
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("integration_test.localPayload", "v1", "default"); err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), localPayload{Value: 7})
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}

	select {
	case msg := <-got:
		if msg.TypeName != "integration_test.localPayload" {
			t.Fatalf("type_name=%q", msg.TypeName)
		}
		if msg.SchemaVersion != "v1" {
			t.Fatalf("schema_version=%q", msg.SchemaVersion)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for payload")
	}

	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventMessageRoutedExact {
				return true
			}
		}
		return false
	}, "expected message_routed_exact event")
}
