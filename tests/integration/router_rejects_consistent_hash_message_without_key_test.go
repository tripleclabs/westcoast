package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestRouterRejectsConsistentHashMessageWithoutKey(t *testing.T) {
	rt := actor.NewRuntime()
	createRouterWorker(t, rt, "hash-invalid-w1", make(chan string, 1))
	router := createRouterActor(t, rt, "router-hash-invalid")
	if err := router.ConfigureRouter(actor.RouterStrategyConsistentKey, []string{"hash-invalid-w1"}); err != nil {
		t.Fatalf("configure router: %v", err)
	}
	ack := router.Route(context.Background(), routerPlainMsg{N: 1})
	if ack.Result == actor.SubmitAccepted {
		t.Fatal("expected consistent-hash rejection for missing hash key")
	}

	outs := router.RoutingOutcomes()
	if len(outs) == 0 {
		t.Fatal("expected routing outcome")
	}
	last := outs[len(outs)-1]
	if last.Outcome != actor.RouteFailedInvalidKey {
		t.Fatalf("expected invalid key outcome, got %s", last.Outcome)
	}
}
