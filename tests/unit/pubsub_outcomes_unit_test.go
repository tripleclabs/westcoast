package unit_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerOutcomesIncludePublishAndNoopUnsubscribe(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-outcome-unit")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := rt.CreateActor("outcome-sub", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := sub.PID("default")
	if err != nil {
		t.Fatal(err)
	}

	_, err = rt.BrokerUnsubscribe(context.Background(), broker.ID(), pid, "unknown.topic", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = rt.BrokerPublish(context.Background(), broker.ID(), "unknown.topic", "payload", "")

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		outs := broker.BrokerOutcomes()
		if hasBrokerOutcome(outs, actor.BrokerOutcomeNotFoundNoop) && hasBrokerOutcome(outs, actor.BrokerOutcomePublishSuccess) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected broker outcomes not_found_noop + publish_success, got %#v", broker.BrokerOutcomes())
}
