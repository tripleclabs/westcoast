package unit_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterOutcomeStoreQuerying(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 8)
	_, _ = rt.CreateActor("store-w1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		hits <- "store-w1"
		return state, nil
	})
	router, _ := rt.CreateActor("store-router", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"store-w1"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		ack := router.Route(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	_ = <-hits
	_ = <-hits
	_ = <-hits

	outs := router.RoutingOutcomes()
	if len(outs) < 3 {
		t.Fatalf("expected at least 3 routing outcomes, got %d", len(outs))
	}
	for _, out := range outs[len(outs)-3:] {
		if out.RouterID != "store-router" {
			t.Fatalf("unexpected router id in outcome: %+v", out)
		}
		if out.Outcome != actor.RouteSuccess {
			t.Fatalf("expected route_success, got %+v", out)
		}
	}
}
