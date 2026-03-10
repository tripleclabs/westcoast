package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRouterPoolRecoveryResumesRouting(t *testing.T) {
	rt := actor.NewRuntime()
	hits := make(chan string, 16)
	createRouterWorker(t, rt, "recover-w1", hits)
	router := createRouterActor(t, rt, "router-recovery")
	if err := router.ConfigureRouter(actor.RouterStrategyRoundRobin, []string{"recover-w1", "recover-missing"}); err != nil {
		t.Fatalf("configure router: %v", err)
	}

	first := router.Route(context.Background(), routerPlainMsg{N: 1})
	second := router.Route(context.Background(), routerPlainMsg{N: 2})
	if first.Result != actor.SubmitAccepted {
		t.Fatalf("expected first route accepted, got %s", first.Result)
	}
	if second.Result == actor.SubmitAccepted {
		t.Fatal("expected second route rejected while missing worker unavailable")
	}

	createRouterWorker(t, rt, "recover-missing", hits)
	third := router.Route(context.Background(), routerPlainMsg{N: 3})
	fourth := router.Route(context.Background(), routerPlainMsg{N: 4})
	if third.Result != actor.SubmitAccepted || fourth.Result != actor.SubmitAccepted {
		t.Fatalf("expected resumed routing acceptance, got third=%s fourth=%s", third.Result, fourth.Result)
	}
}
