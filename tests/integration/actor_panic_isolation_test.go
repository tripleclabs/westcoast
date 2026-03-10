package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestActorPanicIsolation(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1}))
	a, err := rt.CreateActor("iso-a", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	b, err := rt.CreateActor("iso-b", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	assertActorLivenessAfterPeerPanic(t, rt, b, a)

	if a.Status() != actor.ActorRunning {
		t.Fatalf("actor a status=%s", a.Status())
	}
	if b.Status() != actor.ActorRunning {
		t.Fatalf("actor b status=%s", b.Status())
	}
	_ = a.Send(context.Background(), inc{N: 1})
}
