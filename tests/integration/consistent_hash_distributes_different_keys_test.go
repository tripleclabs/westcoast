package integration_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestConsistentHashDistributesDifferentKeys(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 128)
	workers := []string{"hashd-w1", "hashd-w2", "hashd-w3", "hashd-w4"}
	for _, id := range workers {
		createRouterWorker(t, rt, id, hits)
	}
	router := createRouterActor(t, rt, "router-hash-diff")
	if err := router.ConfigureRouter(actor.RouterStrategyConsistentKey, workers); err != nil {
		t.Fatalf("configure consistent hash: %v", err)
	}

	for i := 0; i < 64; i++ {
		ack := router.Route(context.Background(), routerHashMsg{Key: fmt.Sprintf("user-%d", i), N: i})
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	counts := toCountMap(drainHits(hits, 64))
	if len(counts) < 2 {
		t.Fatalf("expected key spread across at least 2 workers, got %v", counts)
	}
}
