package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLifecycleSyncSupervisionTerminalUnregister(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 0, OnLimit: actor.DecisionStop}))
	ref, _ := rt.CreateActor("ls2", 0, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if msg.Payload == "panic" {
			panic("boom")
		}
		return state, nil
	})
	_, _ = ref.RegisterName("life-terminal")
	_ = ref.Send(context.Background(), "panic")
	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorStopped }, "expected actor stopped")
	assertLookupMiss(t, rt.LookupName("life-terminal"))
}
