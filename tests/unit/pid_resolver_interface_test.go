package unit_test

import (
	"testing"

	"westcoast/src/actor"
)

func TestResolverSwapInvariants(t *testing.T) {
	var r actor.PIDResolver = actor.NewInMemoryPIDResolver()
	pid := actor.PID{Namespace: "n", ActorID: "a", Generation: 1}
	r.Register(pid)
	entry, ok := r.Resolve(pid)
	if !ok || entry.CurrentGeneration != 1 || entry.RouteState != actor.PIDRouteReachable {
		t.Fatalf("bad resolver entry: %+v %v", entry, ok)
	}
}
