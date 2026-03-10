package unit_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBrokerSubscribeRejectsInvalidPattern(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-pattern-validation")
	if err != nil {
		t.Fatal(err)
	}
	ref, err := rt.CreateActor("subscriber-pattern-validation", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := ref.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	res, askErr := broker.Ask(context.Background(), actor.BrokerSubscribeCommand{Subscriber: pid, Pattern: "audit.#.tail"}, time.Second)
	if askErr != nil {
		t.Fatal(askErr)
	}
	ack := res.Payload.(actor.BrokerCommandAck)
	if ack.Result != actor.BrokerOutcomeInvalidPattern {
		t.Fatalf("expected invalid_pattern, got %s", ack.Result)
	}
}
