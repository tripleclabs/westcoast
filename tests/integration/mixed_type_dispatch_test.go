package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type mixedV1 struct{}

func (mixedV1) SchemaVersion() string { return "v1" }

type mixedV2 struct{}

func (mixedV2) SchemaVersion() string { return "v2" }

func TestMixedTypeDispatchRoutingBehavior(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	ref, err := rt.CreateActor("mixed", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("integration_test.mixedV1", "v1", "exact"); err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterFallbackRoute("fallback"); err != nil {
		t.Fatal(err)
	}

	if ack := ref.Send(context.Background(), mixedV1{}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("v1 result=%s", ack.Result)
	}
	if ack := ref.Send(context.Background(), mixedV2{}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("v2 result=%s", ack.Result)
	}

	waitFor(t, time.Second, func() bool {
		var exact, fallback bool
		for _, e := range emitter.Events() {
			if e.Type == actor.EventMessageRoutedExact {
				exact = true
			}
			if e.Type == actor.EventMessageRoutedFallback {
				fallback = true
			}
		}
		return exact && fallback
	}, "expected exact and fallback route events")
}
