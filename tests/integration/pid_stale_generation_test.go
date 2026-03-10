package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func restartingHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	if _, ok := msg.Payload.(panicMsg); ok {
		return state, errors.New("boom")
	}
	return pidCounter(context.Background(), state, msg)
}

func TestPIDStaleGenerationRejected(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 2}))
	ref, _ := rt.CreateActor("stale", 0, restartingHandler)
	pid, _ := ref.PID("default")
	_ = ref.Send(context.Background(), panicMsg{})
	time.Sleep(50 * time.Millisecond)
	ack := ref.SendPID(context.Background(), pid, pidInc{N: 1})
	if ack.Outcome != actor.PIDRejectedStaleGeneration {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
}
