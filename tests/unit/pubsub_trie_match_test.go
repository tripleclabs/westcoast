package unit_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerWildcardMatchIsDeterministic(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-trie-determinism")
	if err != nil {
		t.Fatal(err)
	}

	deliveries := make(chan actor.BrokerPublishedMessage, 8)
	sub, err := rt.CreateActor("trie-subscriber", nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if event, ok := msg.Payload.(actor.BrokerPublishedMessage); ok {
			deliveries <- event
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	pid, err := sub.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	_, err = broker.Ask(context.Background(), actor.BrokerSubscribeCommand{Subscriber: pid, Pattern: "user.+.updated"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_, err = broker.Ask(context.Background(), actor.BrokerSubscribeCommand{Subscriber: pid, Pattern: "audit.#"}, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	ack := broker.Send(context.Background(), actor.BrokerPublishCommand{Topic: "user.profile.updated", Payload: "one"})
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("publish ack=%s", ack.Result)
	}
	ack = broker.Send(context.Background(), actor.BrokerPublishCommand{Topic: "audit.security.login", Payload: "two"})
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("publish ack=%s", ack.Result)
	}

	got := map[string]bool{}
	timeout := time.After(time.Second)
	for len(got) < 2 {
		select {
		case m := <-deliveries:
			got[m.Topic] = true
		case <-timeout:
			t.Fatalf("timed out waiting for wildcard deliveries: %#v", got)
		}
	}
	if !got["user.profile.updated"] || !got["audit.security.login"] {
		t.Fatalf("missing expected deliveries: %#v", got)
	}
}
