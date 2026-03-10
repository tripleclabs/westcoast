package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestPIDOnlyPolicyRejectsNonPIDCrossActorInteraction(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	a, err := rt.CreateActor("pida", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.CreateActor("pidb", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil }); err != nil {
		t.Fatal(err)
	}
	ack := a.CrossSendActorID(context.Background(), "pidb", "blocked")
	if ack.Result != actor.SubmitRejectedFound {
		t.Fatalf("result=%s", ack.Result)
	}
	assertGuardrailOutcome(t, a.GuardrailOutcomes(), actor.GuardrailPolicyRejectNonPID)
}
