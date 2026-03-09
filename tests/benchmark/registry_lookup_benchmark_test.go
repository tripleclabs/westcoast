package benchmark_test

import (
	"context"
	"fmt"
	"testing"

	"westcoast/src/actor"
)

func BenchmarkRegistryLookup(b *testing.B) {
	rt := actor.NewRuntime()
	for i := 0; i < 10000; i++ {
		id := fmt.Sprintf("lookup-%d", i)
		ref, err := rt.CreateActor(id, 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
		if err != nil {
			b.Fatal(err)
		}
		if _, err := ref.RegisterName("svc-" + id); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ack := rt.LookupName(fmt.Sprintf("svc-lookup-%d", i%10000))
		if ack.Result != actor.RegistryLookupHit {
			b.Fatalf("lookup result=%s", ack.Result)
		}
	}
}
