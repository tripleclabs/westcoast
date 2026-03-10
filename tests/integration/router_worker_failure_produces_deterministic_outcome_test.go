package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterWorkerFailureProducesDeterministicOutcome(t *testing.T) {
	rt := actor.NewRuntime()
	router := createRouterActor(t, rt, "router-worker-failure")
	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"missing-worker"}); err != nil {
		t.Fatalf("configure router: %v", err)
	}
	ack := router.Route(context.Background(), routerPlainMsg{N: 1})
	if ack.Result == actor.SubmitAccepted {
		t.Fatal("expected rejection for unavailable worker")
	}
	outs := router.RoutingOutcomes()
	if len(outs) == 0 {
		t.Fatal("expected routing outcome")
	}
	if got := outs[len(outs)-1].Outcome; got != actor.RouteFailedWorkerUnavailable {
		t.Fatalf("expected route_failed_worker_unavailable, got %s", got)
	}
}
