package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestPIDStoppedOutcome(t *testing.T) {
	rt := actor.NewRuntime()
	ref, _ := rt.CreateActor("stopped", 0, pidCounter)
	pid, _ := ref.PID("default")
	_ = ref.Stop()
	ack := ref.SendPID(context.Background(), pid, pidInc{N: 1})
	if ack.Outcome != actor.PIDRejectedStopped {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
}
