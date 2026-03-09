package contract_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestPIDGatewayGuardrailsContract(t *testing.T) {
	vals := []actor.GuardrailOutcomeType{
		actor.GuardrailPolicyAccept,
		actor.GuardrailPolicyRejectNonPID,
		actor.GuardrailGatewayRouteSuccess,
		actor.GuardrailGatewayRouteFailure,
	}
	for _, v := range vals {
		if string(v) == "" {
			t.Fatal("empty guardrail outcome")
		}
	}
}

func TestPIDPolicyAcceptRejectContract(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	a, err := rt.CreateActor("cga", 0, noop)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rt.CreateActor("cgb", 0, noop); err != nil {
		t.Fatal(err)
	}
	ack := a.CrossSendActorID(context.Background(), "cgb", "x")
	if ack.Result != actor.SubmitRejectedFound {
		t.Fatalf("expected policy rejection, got=%s", ack.Result)
	}
	outs := a.GuardrailOutcomes()
	if len(outs) == 0 || outs[len(outs)-1].Outcome != actor.GuardrailPolicyRejectNonPID {
		t.Fatal("expected policy_reject_non_pid outcome")
	}
}

func TestGatewayModeContract(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	rt.SetGatewayRouteMode(actor.GatewayRouteGatewayMediated)
	rt.SetGatewayAvailable(false)
	ref, err := rt.CreateActor("cgw", 0, noop)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := ref.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.CrossSendPID(context.Background(), pid, "x")
	if ack.Outcome != actor.PIDUnresolved {
		t.Fatalf("expected unresolved on unavailable gateway, got=%s", ack.Outcome)
	}
}

func TestReadinessScopeContract(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	records := rt.ValidateDistributedReadiness()
	if len(records) < 3 {
		t.Fatalf("records=%d want>=3", len(records))
	}
}
