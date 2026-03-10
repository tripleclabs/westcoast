package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterRoundRobinDistributesAcrossWorkers(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 16)
	createRouterWorker(t, rt, "rr-w1", hits)
	createRouterWorker(t, rt, "rr-w2", hits)
	createRouterWorker(t, rt, "rr-w3", hits)
	router := createRouterActor(t, rt, "router-rr")

	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"rr-w1", "rr-w2", "rr-w3"}); err != nil {
		t.Fatalf("configure router: %v", err)
	}
	for i := 0; i < 6; i++ {
		ack := router.Route(context.Background(), routerPlainMsg{N: i})
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	got := drainHits(hits, 6)
	_ = got
	want := []string{"rr-w1", "rr-w2", "rr-w3", "rr-w1", "rr-w2", "rr-w3"}

	outs := router.RoutingOutcomes()
	if len(outs) < 6 {
		t.Fatalf("expected >=6 routing outcomes, got %d", len(outs))
	}
	last := outs[len(outs)-6:]
	for i := range want {
		if last[i].SelectedWorker != want[i] {
			t.Fatalf("round-robin mismatch at %d: got %s want %s", i, last[i].SelectedWorker, want[i])
		}
	}
}
