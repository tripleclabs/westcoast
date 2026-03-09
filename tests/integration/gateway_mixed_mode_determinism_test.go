package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestGatewayMixedModeDeterminism(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	s, _ := rt.CreateActor("mix-s", 0, pidCounter)
	r, _ := rt.CreateActor("mix-r", 0, pidCounter)
	pid, _ := r.PID("default")

	rt.SetGatewayRouteMode(actor.GatewayRouteLocalDirect)
	ack1 := s.CrossSendPID(context.Background(), pid, pidInc{N: 1})
	if ack1.Outcome != actor.PIDDelivered {
		t.Fatalf("direct outcome=%s", ack1.Outcome)
	}

	rt.SetGatewayRouteMode(actor.GatewayRouteGatewayMediated)
	rt.SetGatewayAvailable(true)
	ack2 := s.CrossSendPID(context.Background(), pid, pidInc{N: 1})
	if ack2.Outcome != actor.PIDDelivered {
		t.Fatalf("mediated outcome=%s", ack2.Outcome)
	}
}
