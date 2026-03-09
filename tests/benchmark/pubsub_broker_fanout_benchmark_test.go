package benchmark_test

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"westcoast/src/actor"
)

const (
	defaultPubSubFanout   = 20
	defaultPubSubInflight = 256
)

func pubSubBenchFanout() int {
	if raw := os.Getenv("WC_PUBSUB_BENCH_FANOUT"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultPubSubFanout
}

func pubSubBenchInflight() int {
	if raw := os.Getenv("WC_PUBSUB_BENCH_INFLIGHT"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultPubSubInflight
}

func BenchmarkPubSubBrokerFanout(b *testing.B) {
	rt := actor.NewRuntime()
	broker, err := rt.EnsureBrokerActor("broker-bench")
	if err != nil {
		b.Fatal(err)
	}
	fanout := pubSubBenchFanout()
	inflight := pubSubBenchInflight()
	for i := 0; i < fanout; i++ {
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

	accepted := 0
	completed := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for {
			if accepted-completed >= inflight {
				completed = broker.BrokerPublishedCount()
				runtime.Gosched()
				continue
			}
			ack := rt.BrokerPublish(context.Background(), broker.ID(), "bench.topic", i, "bench-publisher")
			if ack.Result == actor.SubmitAccepted {
				accepted++
				break
			}
			if ack.Result != actor.SubmitRejectedFull {
				b.Fatalf("publish rejected: %s", ack.Result)
			}
			runtime.Gosched()
		}
	}
	b.StopTimer()

	// Ensure async publish completions catch up before benchmark exits.
	deadline := time.Now().Add(10 * time.Second)
	for completed < accepted {
		if time.Now().After(deadline) {
			b.Fatalf("timed out waiting for publish completion: accepted=%d completed=%d", accepted, completed)
		}
		completed = broker.BrokerPublishedCount()
		runtime.Gosched()
	}

	b.ReportMetric(float64(fanout), "fanout")
	b.ReportMetric(float64(inflight), "inflight")
}
