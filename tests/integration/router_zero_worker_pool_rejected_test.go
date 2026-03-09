package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestRouterZeroWorkerPoolRejected(t *testing.T) {
	rt := actor.NewRuntime()
	router := createRouterActor(t, rt, "router-zero-workers")
	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, nil); err != nil {
		t.Fatalf("configure router: %v", err)
	}
	ack := router.Route(context.Background(), routerPlainMsg{N: 1})
	if ack.Result == actor.SubmitAccepted {
		t.Fatal("expected rejection for zero-worker pool")
	}
	outs := router.RoutingOutcomes()
	if len(outs) == 0 || outs[len(outs)-1].Outcome != actor.RouteFailedNoWorkers {
		t.Fatalf("expected route_failed_no_workers, got %+v", outs)
	}
}
