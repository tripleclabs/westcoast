package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerDynamicUnsubscribeViaAsk(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-dynamic-unsubscribe")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 1)
	sub, err := rt.CreateActor("dynamic-unsubscribe-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := sub.PID("default")
	if _, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "inventory.updated", time.Second); err != nil {
		t.Fatal(err)
	}
	res, askErr := broker.Ask(context.Background(), actor.BrokerUnsubscribeCommand{
		Subscriber: pid,
		Pattern:    "inventory.updated",
	}, time.Second)
	if askErr != nil {
		t.Fatal(askErr)
	}
	ack := res.Payload.(actor.BrokerCommandAck)
	if ack.Result != actor.BrokerOutcomeUnsubscribeSuccess {
		t.Fatalf("unsubscribe result=%s", ack.Result)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "inventory.updated", "nope", "publisher")
	select {
	case <-ch:
		t.Fatal("unexpected delivery after unsubscribe")
	case <-time.After(200 * time.Millisecond):
	}
}
