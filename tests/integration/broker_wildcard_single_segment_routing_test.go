package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBrokerWildcardSingleSegmentRouting(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-wildcard-single")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 4)
	sub, err := rt.CreateActor("wildcard-single-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := sub.PID("default")
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "user.+.updated", time.Second); err != nil {
		t.Fatal(err)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "user.profile.updated", "a", "publisher")
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "user.billing.updated", "b", "publisher")

	got := map[string]bool{}
	deadline := time.After(time.Second)
	for len(got) < 2 {
		select {
		case ev := <-ch:
			got[ev.Topic] = true
		case <-deadline:
			t.Fatalf("missing wildcard deliveries: %#v", got)
		}
	}
}
