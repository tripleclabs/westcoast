package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestGatewayBoundaryRoutingFailureIsDeterministic(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	rt.SetGatewayRouteMode(actor.GatewayRouteGatewayMediated)
	rt.SetGatewayAvailable(false)
	sender, _ := rt.CreateActor("gwf-s", 0, pidCounter)
	recv, _ := rt.CreateActor("gwf-r", 0, pidCounter)
	pid, _ := recv.PID("default")

	ack := sender.CrossSendPID(context.Background(), pid, pidInc{N: 1})
	if ack.Outcome != actor.PIDUnresolved {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
	assertGuardrailOutcome(t, sender.GuardrailOutcomes(), actor.GuardrailGatewayRouteFailure)
}
