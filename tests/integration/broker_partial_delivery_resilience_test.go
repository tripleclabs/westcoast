package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerPartialDeliveryResilience(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-partial-delivery")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 1)
	live, err := rt.CreateActor("partial-live-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	livePID, _ := live.PID("default")
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), livePID, "ops.alert", time.Second); err != nil {
		t.Fatal(err)
	}
	// Deliberately subscribe an unreachable PID to force partial delivery.
	deadPID := actor.PID{Namespace: "default", ActorID: "missing-actor", Generation: 1}
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), deadPID, "ops.alert", time.Second); err != nil {
		t.Fatal(err)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "ops.alert", "critical", "publisher")

	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected live subscriber delivery")
	}
	waitForBrokerOutcome(t, broker, time.Second, actor.BrokerOutcomePublishPartialDelivery)
}
