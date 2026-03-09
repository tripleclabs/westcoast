package contract_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func panicOnPanicMsg(_ context.Context, state any, msg actor.Message) (any, error) {
	switch msg.Payload.(type) {
	case struct{ Panic bool }:
		panic("boom")
	}
	return state, nil
}

func TestSupervisionDecisionOutcomeSet(t *testing.T) {
	vals := []actor.SupervisionDecision{actor.DecisionRestart, actor.DecisionStop, actor.DecisionEscalate}
	for _, v := range vals {
		if string(v) == "" {
			t.Fatal("empty decision")
		}
	}
}

func TestSupervisionDefaultDeterminism(t *testing.T) {
	s := actor.DefaultSupervisor{MaxRestarts: 2, OnLimit: actor.DecisionEscalate}
	if d := s.Decide("a", nil, 0); d != actor.DecisionRestart {
		t.Fatalf("d0=%s", d)
	}
	if d := s.Decide("a", nil, 1); d != actor.DecisionRestart {
		t.Fatalf("d1=%s", d)
	}
	if d := s.Decide("a", nil, 2); d != actor.DecisionEscalate {
		t.Fatalf("d2=%s", d)
	}
}

func TestActorLocalCrashInterceptionContract(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1}))
	ref, err := rt.CreateActor("c-super", 0, panicOnPanicMsg)
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), struct{ Panic bool }{Panic: true})
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, ok := rt.Outcome(ack.MessageID); ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("missing terminal outcome")
}
