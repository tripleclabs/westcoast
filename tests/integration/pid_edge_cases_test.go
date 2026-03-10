package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestPIDEdgeCases(t *testing.T) {
	rt := actor.NewRuntime()
	ref, _ := rt.CreateActor("edge-pid", 0, pidCounter)
	_, _ = ref.PID("default")
	unknown := actor.PID{Namespace: "default", ActorID: "missing", Generation: 1}
	ack := ref.SendPID(context.Background(), unknown, pidInc{N: 1})
	if ack.Outcome != actor.PIDRejectedNotFound {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
}
