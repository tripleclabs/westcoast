package unit_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type routeV1 struct{ ID int }

func (routeV1) SchemaVersion() string { return "v1" }

type routeV2 struct{ ID int }

func (routeV2) SchemaVersion() string { return "v2" }

func waitFor(t *testing.T, timeout time.Duration, condition func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(msg)
}

func TestTypeRoutingPrecedenceExactBeforeFallback(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	ref, err := rt.CreateActor("r-precedence", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("unit_test.routeV1", "v1", "exact"); err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterFallbackRoute("fallback"); err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), routeV1{ID: 1}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}

	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventMessageRoutedExact {
				return true
			}
		}
		return false
	}, "expected exact route event")
}

func TestTypeRoutingFallbackUsedWhenExactMissing(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter))
	ref, err := rt.CreateActor("r-fallback", nil, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("unit_test.routeV1", "v1", "exact"); err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterFallbackRoute("fallback"); err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), routeV2{ID: 1}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}

	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventMessageRoutedFallback {
				return true
			}
		}
		return false
	}, "expected fallback route event")
}
