package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestGatewayBoundaryModeKeepsBusinessLogicUnchanged(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	rt.SetGatewayRouteMode(actor.GatewayRouteGatewayMediated)
	rt.SetGatewayAvailable(true)
	sender, _ := rt.CreateActor("gw-s", 0, pidCounter)
	recv, _ := rt.CreateActor("gw-r", 0, pidCounter)
	pid, _ := recv.PID("default")

	ack := sender.CrossSendPID(context.Background(), pid, pidInc{N: 2})
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
	ch := make(chan int, 1)
	_ = recv.Send(context.Background(), pidGet{Ch: ch})
	select {
	case got := <-ch:
		if got != 2 {
			t.Fatalf("got=%d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	assertGuardrailOutcome(t, sender.GuardrailOutcomes(), actor.GuardrailGatewayRouteSuccess)
}
