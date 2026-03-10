package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBrokerWildcardNonMatchReceivesNothing(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-wildcard-nonmatch")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 1)
	sub, err := rt.CreateActor("wildcard-nonmatch-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := sub.PID("default")
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "user.*.updated", time.Second); err != nil {
		t.Fatal(err)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "billing.invoice.created", "payload", "publisher")
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event: %#v", ev)
	case <-time.After(200 * time.Millisecond):
	}
}
