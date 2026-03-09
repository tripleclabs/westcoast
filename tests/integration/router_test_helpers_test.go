package integration_test

import (
	"context"
	"sync"
	"testing"

	"westcoast/src/actor"
)

type routerHashMsg struct {
	Key string
	N   int
}

func (m routerHashMsg) HashKey() string { return m.Key }

type routerPlainMsg struct {
	N int
}

func createRouterWorker(t *testing.T, rt *actor.Runtime, workerID string, hits chan<- string) {
	t.Helper()
	_, err := rt.CreateActor(workerID, 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		hits <- workerID
		return state, nil
	})
	if err != nil {
		t.Fatalf("create worker %s: %v", workerID, err)
	}
}

func drainHits(hits <-chan string, n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, <-hits)
	}
	return out
}

func toCountMap(ids []string) map[string]int {
	counts := make(map[string]int, len(ids))
	for _, id := range ids {
		counts[id]++
	}
	return counts
}

func createRouterActor(t *testing.T, rt *actor.Runtime, id string) *actor.ActorRef {
	t.Helper()
	ref, err := rt.CreateActor(id, 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatalf("create router actor: %v", err)
	}
	return ref
}

func sendConcurrentRoutes(t *testing.T, router *actor.ActorRef, n int, payload any) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			ack := router.Route(context.Background(), payload)
			if ack.Result != actor.SubmitAccepted {
				t.Errorf("route rejected: %s", ack.Result)
			}
		}()
	}
	wg.Wait()
}
