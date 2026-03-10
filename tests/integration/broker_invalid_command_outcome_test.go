package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBrokerInvalidCommandOutcome(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-invalid-command")
	if err != nil {
		t.Fatal(err)
	}
	_, askErr := broker.Ask(context.Background(), "not-a-command", time.Second)
	if askErr != nil {
		t.Fatal(askErr)
	}
	waitForBrokerOutcome(t, broker, time.Second, actor.BrokerOutcomeInvalidCommand)
}
