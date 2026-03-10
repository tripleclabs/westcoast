package unit_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterRoundRobinCounterConcurrentBalance(t *testing.T) {
	rt := actor.NewRuntime()
	var mu sync.Mutex
	counts := map[string]int{}
	workers := []string{"rrc-w1", "rrc-w2", "rrc-w3"}
	for _, id := range workers {
		workerID := id
		_, err := rt.CreateActor(workerID, 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
			mu.Lock()
			counts[workerID]++
			mu.Unlock()
			return state, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	router, err := rt.CreateActor("router-rr-counter", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, workers); err != nil {
		t.Fatal(err)
	}

	const total = 300
	var wg sync.WaitGroup
	wg.Add(total)
	for i := 0; i < total; i++ {
		go func() {
			defer wg.Done()
			ack := router.Route(context.Background(), i)
			if ack.Result != actor.SubmitAccepted {
				t.Errorf("route rejected: %s", ack.Result)
			}
		}()
	}
	wg.Wait()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		processed := 0
		for _, c := range counts {
			processed += c
		}
		mu.Unlock()
		if processed == total {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	processed := 0
	min, max := total, 0
	for _, id := range workers {
		c := counts[id]
		processed += c
		if c < min {
			min = c
		}
		if c > max {
			max = c
		}
	}
	if processed != total {
		t.Fatalf("expected %d processed messages, got %d (%+v)", total, processed, counts)
	}
	if max-min > 1 {
		t.Fatalf("expected balanced round-robin counts, got %+v", counts)
	}
}
