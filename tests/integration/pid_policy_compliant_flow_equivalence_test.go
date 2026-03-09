package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestPIDCompliantFlowRemainsBehaviorallyEquivalent(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	sender, err := rt.CreateActor("eq-s", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}
	recv, err := rt.CreateActor("eq-r", 0, pidCounter)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := recv.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	ack := sender.CrossSendPID(context.Background(), pid, pidInc{N: 4})
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
	ch := make(chan int, 1)
	_ = recv.Send(context.Background(), pidGet{Ch: ch})
	select {
	case got := <-ch:
		if got != 4 {
			t.Fatalf("got=%d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	assertGuardrailOutcome(t, sender.GuardrailOutcomes(), actor.GuardrailPolicyAccept)
}
