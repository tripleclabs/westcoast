package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestPIDResolverSwapContractStability(t *testing.T) {
	rt := actor.NewRuntime()
	ref, _ := rt.CreateActor("swap", 0, pidCounter)
	pid, _ := ref.PID("default")
	ack := ref.SendPID(context.Background(), pid, pidInc{N: 2})
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
}
