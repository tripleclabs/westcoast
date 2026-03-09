package contract_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestNativePubSubBrokerContract(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-contract")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := rt.CreateActor("broker-contract-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := sub.PID("default")
	if err != nil {
		t.Fatal(err)
	}

	ack, err := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "user.+.updated", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Result != actor.BrokerOutcomeSubscribeSuccess {
		t.Fatalf("subscribe result=%s", ack.Result)
	}

	ack, err = rt.BrokerUnsubscribe(context.Background(), broker.ID(), pid, "missing.topic", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if ack.Result != actor.BrokerOutcomeNotFoundNoop {
		t.Fatalf("unsubscribe missing result=%s", ack.Result)
	}

	_ = rt.BrokerPublish(context.Background(), broker.ID(), "user.profile.updated", map[string]string{"id": "1"}, "publisher")
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		outcomes := broker.BrokerOutcomes()
		foundSub := false
		foundNoop := false
		foundPub := false
		for _, out := range outcomes {
			switch out.Result {
			case actor.BrokerOutcomeSubscribeSuccess:
				foundSub = true
			case actor.BrokerOutcomeNotFoundNoop:
				foundNoop = true
			case actor.BrokerOutcomePublishSuccess, actor.BrokerOutcomePublishPartialDelivery:
				foundPub = true
			}
		}
		if foundSub && foundNoop && foundPub {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("missing expected contract outcomes: %#v", broker.BrokerOutcomes())
}
