package integration_test

import (
	"testing"
	"time"

	"westcoast/src/actor"
)

func waitForBrokerOutcome(t *testing.T, broker *actor.ActorRef, timeout time.Duration, want actor.BrokerOutcomeType) actor.BrokerOutcome {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		outs := broker.BrokerOutcomes()
		for i := len(outs) - 1; i >= 0; i-- {
			if outs[i].Result == want {
				return outs[i]
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for broker outcome %s", want)
	return actor.BrokerOutcome{}
}
