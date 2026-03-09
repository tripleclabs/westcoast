package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerSubscriptionIdempotencyAndNoopUnsubscribe(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-idempotency")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 2)
	sub, err := rt.CreateActor("idempotency-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := sub.PID("default")
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "customer.updated", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "customer.updated", time.Second); err != nil {
		t.Fatal(err)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "customer.updated", "v", "publisher")
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("missing delivery")
	}
	select {
	case <-ch:
		t.Fatal("duplicate delivery for idempotent subscribe")
	case <-time.After(150 * time.Millisecond):
	}
	ack, err := rt.BrokerUnsubscribe(context.Background(), broker.ID(), pid, "customer.missing", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Result != actor.BrokerOutcomeNotFoundNoop {
		t.Fatalf("expected not_found_noop, got %s", ack.Result)
	}
}
