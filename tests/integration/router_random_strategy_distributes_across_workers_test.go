package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterRandomStrategyDistributesAcrossWorkers(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 256)
	workers := []string{"rnd-w1", "rnd-w2", "rnd-w3", "rnd-w4"}
	for _, id := range workers {
		createRouterWorker(t, rt, id, hits)
	}
	router := createRouterActor(t, rt, "router-random")
	if err := router.ConfigureRouter(actor.RouterStrategyRandom, workers); err != nil {
		t.Fatalf("configure random router: %v", err)
	}

	for i := 0; i < 200; i++ {
		ack := router.Route(context.Background(), routerPlainMsg{N: i})
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	counts := toCountMap(drainHits(hits, 200))
	if len(counts) < 2 {
		t.Fatalf("expected distribution across at least 2 workers, got %v", counts)
	}
}
