package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestRestartLimitDeterminism(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(
		actor.WithEmitter(emitter),
		actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1, OnLimit: actor.DecisionEscalate}),
	)
	ref, err := rt.CreateActor("limit", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	if ack := ref.Send(context.Background(), panicMsg{}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("first panic send=%s", ack.Result)
	}
	if ack := ref.Send(context.Background(), panicMsg{}); ack.Result != actor.SubmitAccepted {
		t.Fatalf("second panic send=%s", ack.Result)
	}
	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventActorEscalated && e.ActorID == "limit" {
				return true
			}
		}
		return false
	}, "expected escalation event")

	waitFor(t, time.Second, func() bool {
		return ref.Status() == actor.ActorStopped
	}, "expected actor stopped after escalation")
}
