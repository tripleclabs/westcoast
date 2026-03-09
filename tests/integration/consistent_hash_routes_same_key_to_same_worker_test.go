package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestConsistentHashRoutesSameKeyToSameWorker(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 32)
	workers := []string{"hash-w1", "hash-w2", "hash-w3"}
	for _, id := range workers {
		createRouterWorker(t, rt, id, hits)
	}
	router := createRouterActor(t, rt, "router-hash-same")
	if err := router.ConfigureRouter(actor.RouterStrategyConsistentKey, workers); err != nil {
		t.Fatalf("configure consistent hash: %v", err)
	}

	for i := 0; i < 20; i++ {
		ack := router.Route(context.Background(), routerHashMsg{Key: "user-1", N: i})
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	first := <-hits
	for i := 1; i < 20; i++ {
		if got := <-hits; got != first {
			t.Fatalf("same key routed to different workers: %s vs %s", first, got)
		}
	}
}
