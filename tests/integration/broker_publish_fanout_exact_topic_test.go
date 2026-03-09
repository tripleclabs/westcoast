package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestBrokerPublishFanOutToExactSubscribers(t *testing.T) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-exact-fanout")
	if err != nil {
		t.Fatal(err)
	}

	type subState struct {
		ch chan actor.BrokerPublishedMessage
	}
	newSub := func(id string) (*actor.ActorRef, chan actor.BrokerPublishedMessage) {
		ch := make(chan actor.BrokerPublishedMessage, 1)
		ref, createErr := rt.CreateActor(id, subState{ch: ch}, func(ctx context.Context, state any, msg actor.Message) (any, error) {
			st := state.(subState)
			st.ch <- msg.Payload.(actor.BrokerPublishedMessage)
			return st, nil
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		return ref, ch
	}
	s1, c1 := newSub("exact-sub-1")
	s2, c2 := newSub("exact-sub-2")
	p1, _ := s1.PID("default")
	p2, _ := s2.PID("default")
	if _, err = rt.BrokerSubscribe(context.Background(), broker.ID(), p1, "order.created", time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err = rt.BrokerSubscribe(context.Background(), broker.ID(), p2, "order.created", time.Second); err != nil {
		t.Fatal(err)
	}

	ack := rt.BrokerPublish(context.Background(), broker.ID(), "order.created", "payload", "publisher")
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("publish ack=%s", ack.Result)
	}

	select {
	case msg := <-c1:
		if msg.Topic != "order.created" {
			t.Fatalf("unexpected topic %s", msg.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber 1 did not receive message")
	}
	select {
	case msg := <-c2:
		if msg.Topic != "order.created" {
			t.Fatalf("unexpected topic %s", msg.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber 2 did not receive message")
	}
	waitForBrokerOutcome(t, broker, time.Second, actor.BrokerOutcomePublishSuccess)
}
