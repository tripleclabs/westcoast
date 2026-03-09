package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestPanicFailureEventEmission(t *testing.T) {
	emitter := actor.NewMemoryEmitter()
	rt := actor.NewRuntime(actor.WithEmitter(emitter), actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1}))
	ref, err := rt.CreateActor("evt", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool {
		for _, e := range emitter.Events() {
			if e.Type == actor.EventActorFailed && e.ActorID == "evt" && e.MessageID == ack.MessageID {
				return e.SupervisionDecision == string(actor.DecisionRestart)
			}
		}
		return false
	}, "expected actor_failed event with supervision decision")
}
