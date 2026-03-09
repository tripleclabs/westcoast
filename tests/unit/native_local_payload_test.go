package unit_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type nativePayload struct {
	Value int
}

func (nativePayload) SchemaVersion() string { return "v1" }

type nativeQuery struct {
	ch chan any
}

func TestNativePayloadPassThrough(t *testing.T) {
	rt := actor.NewRuntime()
	var seen any
	ref, err := rt.CreateActor("native", nil, func(_ context.Context, state any, msg actor.Message) (any, error) {
		switch v := msg.Payload.(type) {
		case *nativePayload:
			seen = v
		case nativeQuery:
			v.ch <- seen
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterTypeRoute("*unit_test.nativePayload", "v1", "default"); err != nil {
		t.Fatal(err)
	}
	if err := ref.RegisterFallbackRoute("fallback"); err != nil {
		t.Fatal(err)
	}

	payload := &nativePayload{Value: 42}
	if ack := ref.Send(context.Background(), payload); ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}

	ch := make(chan any, 1)
	if ack := ref.Send(context.Background(), nativeQuery{ch: ch}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("query result=%s", ack.Result)
	}
	select {
	case got := <-ch:
		if got != payload {
			t.Fatal("payload pointer identity changed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting query")
	}
}
