package benchmark_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"westcoast/src/actor"
)

func BenchmarkPubSubBrokerFanout(b *testing.B) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-bench")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("bench-sub-%d", i)
		ref, createErr := rt.CreateActor(id, nil, func(ctx context.Context, state any, msg actor.Message) (any, error) {
			return state, nil
		})
		if createErr != nil {
			b.Fatal(createErr)
		}
		pid, _ := ref.PID("default")
		if _, subErr := rt.BrokerSubscribe(context.Background(), broker.ID(), pid, "bench.topic", time.Second); subErr != nil {
			b.Fatal(subErr)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ack := rt.BrokerPublish(context.Background(), broker.ID(), "bench.topic", i, "bench-publisher")
		if ack.Result != actor.SubmitAccepted {
			b.Fatalf("publish rejected: %s", ack.Result)
		}
	}
}
