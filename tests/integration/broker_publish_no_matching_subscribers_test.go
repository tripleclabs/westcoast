package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestBrokerPublishNoMatchingSubscribers(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-no-match")
	if err != nil {
		t.Fatal(err)
	}
	ack := rt.BrokerPublish(context.Background(), broker.ID(), "topic.none", "payload", "publisher")
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("publish ack=%s", ack.Result)
	}
	out := waitForBrokerOutcome(t, broker, time.Second, actor.BrokerOutcomePublishSuccess)
	if out.MatchedCount != 0 {
		t.Fatalf("expected matched_count=0, got %d", out.MatchedCount)
	}
}
