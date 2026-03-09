package unit_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"westcoast/src/actor"
)

func TestRegistryConcurrentDuplicateDeterminism(t *testing.T) {
	rt := actor.NewRuntime()
	refA, err := rt.CreateActor("ra", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}
	refB, err := rt.CreateActor("rb", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}

	var success atomic.Int64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if ack, err := refA.RegisterName("svc-race"); err == nil && ack.Result == actor.RegistryRegisterSuccess {
			success.Add(1)
		}
	}()
	go func() {
		defer wg.Done()
		if ack, err := refB.RegisterName("svc-race"); err == nil && ack.Result == actor.RegistryRegisterSuccess {
			success.Add(1)
		}
	}()
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("success count=%d want=1", success.Load())
	}
}
