package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerDynamicSubscribeViaAsk(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-dynamic-subscribe")
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan actor.BrokerPublishedMessage, 1)
	sub, err := rt.CreateActor("dynamic-subscribe-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		ch <- msg.Payload.(actor.BrokerPublishedMessage)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := sub.PID("default")
	res, askErr := broker.Ask(context.Background(), actor.BrokerSubscribeCommand{
		Subscriber: pid,
		Pattern:    "payments.completed",
	}, time.Second)
	if askErr != nil {
		t.Fatal(askErr)
	}
	ack := res.Payload.(actor.BrokerCommandAck)
	if ack.Result != actor.BrokerOutcomeSubscribeSuccess {
		t.Fatalf("subscribe result=%s", ack.Result)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "payments.completed", "ok", "publisher")
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("expected delivery after dynamic subscribe")
	}
}
