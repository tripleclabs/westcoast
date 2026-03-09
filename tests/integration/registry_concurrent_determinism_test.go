package integration_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"westcoast/src/actor"
)

func TestConcurrentRegistryDeterminism(t *testing.T) {
	rt := actor.NewRuntime()
	refs := make([]*actor.ActorRef, 16)
	for i := range refs {
		ref, err := rt.CreateActor("crd-"+string(rune('a'+i)), 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
		if err != nil {
			t.Fatal(err)
		}
		refs[i] = ref
	}

	var success atomic.Int64
	var wg sync.WaitGroup
	for _, ref := range refs {
		wg.Add(1)
		go func(ref *actor.ActorRef) {
			defer wg.Done()
			if ack, err := ref.RegisterName("concurrent-shared"); err == nil && ack.Result == actor.RegistryRegisterSuccess {
				success.Add(1)
			}
		}(ref)
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatalf("success count=%d want=1", success.Load())
	}
	if got := rt.LookupName("concurrent-shared"); got.Result != actor.RegistryLookupHit {
		t.Fatalf("lookup result=%s", got.Result)
	}
}
