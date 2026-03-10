package contract_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

type contractHashMsg struct{ key string }

func (m contractHashMsg) HashKey() string { return m.key }

func TestRouterWorkerPoolContractOutcomes(t *testing.T) {
	required := []actor.RoutingOutcomeType{
		actor.RouteSuccess,
		actor.RouteFailedNoWorkers,
		actor.RouteFailedInvalidKey,
		actor.RouteFailedWorkerUnavailable,
	}
	if len(required) != 4 {
		t.Fatalf("expected 4 routing outcomes, got %d", len(required))
	}
}

func TestRouterContractStatelessRouting(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 16)
	_, _ = rt.CreateActor("w1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { hits <- "w1"; return state, nil })
	_, _ = rt.CreateActor("w2", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { hits <- "w2"; return state, nil })
	router, _ := rt.CreateActor("router-contract-stateless", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })

	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"w1", "w2"}); err != nil {
		t.Fatalf("configure router: %v", err)
	}
	for i := 0; i < 4; i++ {
		ack := router.Route(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("route %d rejected: %s", i, ack.Result)
		}
	}
	_ = <-hits
	_ = <-hits
	_ = <-hits
	_ = <-hits
	outs := router.RoutingOutcomes()
	if len(outs) < 4 {
		t.Fatalf("expected >=4 outcomes, got %d", len(outs))
	}
	want := []string{"w1", "w2", "w1", "w2"}
	got := outs[len(outs)-4:]
	for i, out := range got {
		if out.SelectedWorker != want[i] || out.Outcome != actor.RouteSuccess {
			t.Fatalf("round-robin contract violated at %d: got %+v want worker=%s", i, out, want[i])
		}
	}
}

func TestRouterContractConsistentHashAndFailure(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 32)
	_, _ = rt.CreateActor("cw1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { hits <- "cw1"; return state, nil })
	_, _ = rt.CreateActor("cw2", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { hits <- "cw2"; return state, nil })
	router, _ := rt.CreateActor("router-contract-hash", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })

	if err := router.ConfigureRouter(actor.RouterStrategyConsistentKey, []string{"cw1", "cw2"}); err != nil {
		t.Fatalf("configure hash router: %v", err)
	}
	for i := 0; i < 8; i++ {
		ack := router.Route(context.Background(), contractHashMsg{key: "user-42"})
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("hash route rejected: %s", ack.Result)
		}
	}
	first := <-hits
	for i := 1; i < 8; i++ {
		if got := <-hits; got != first {
			t.Fatalf("same hash key mapped to multiple workers: %s vs %s", first, got)
		}
	}

	ack := router.Route(context.Background(), struct{ N int }{N: 1})
	if ack.Result == actor.SubmitAccepted {
		t.Fatal("expected invalid key routing rejection")
	}

	badRouter, _ := rt.CreateActor("router-contract-fail", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err := badRouter.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"missing-worker"}); err != nil {
		t.Fatalf("configure bad router: %v", err)
	}
	ack = badRouter.Route(context.Background(), 1)
	if ack.Result == actor.SubmitAccepted {
		t.Fatal("expected unavailable worker routing rejection")
	}
}
