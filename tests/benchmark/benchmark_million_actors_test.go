package benchmark_test

import (
	"context"
	"fmt"
	"os"
	gruntime "runtime"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

func benchmarkTarget() int {
	if raw := os.Getenv("WC_BENCH_TARGET"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	if os.Getenv("WC_BENCH_PROFILE") == "full" {
		return 1_000_000
	}
	if testing.Short() {
		return 5_000
	}
	return 100_000
}

func envInt(name string, defaultValue int) int {
	if raw := os.Getenv(name); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return defaultValue
}

type latencyRecorder struct {
	sampleEvery uint64
	maxSamples  int
	seen        atomic.Uint64
	mu          sync.Mutex
	samplesNs   []int64
}

func newLatencyRecorder(sampleEvery uint64, maxSamples int) *latencyRecorder {
	if sampleEvery == 0 {
		sampleEvery = 1
	}
	if maxSamples <= 0 {
		maxSamples = 1
	}
	return &latencyRecorder{
		sampleEvery: sampleEvery,
		maxSamples:  maxSamples,
		samplesNs:   make([]int64, 0, maxSamples),
	}
}

func (r *latencyRecorder) ObserveMailboxDepth(string, int)                {}
func (r *latencyRecorder) ObserveProcessingLatency(string, time.Duration) {}
func (r *latencyRecorder) ObserveLocalSendLatency(_ string, d time.Duration) {
	r.ObserveEnqueueLatency("", d)
}
func (r *latencyRecorder) ObserveLocalRouting(string, string)                 {}
func (r *latencyRecorder) ObservePanicIntercept(string)                       {}
func (r *latencyRecorder) ObserveMailboxPreservedDepth(string, int)           {}
func (r *latencyRecorder) ObserveRestart(string)                              {}
func (r *latencyRecorder) ObservePIDLookupLatency(string, time.Duration)      {}
func (r *latencyRecorder) ObserveRegistryLookupLatency(string, time.Duration) {}
func (r *latencyRecorder) ObserveRegistryOperation(string)                    {}
func (r *latencyRecorder) ObserveLifecycleHook(string, string)                {}

func (r *latencyRecorder) ObserveEnqueueLatency(_ string, d time.Duration) {
	n := r.seen.Add(1)
	if n != 1 && n%r.sampleEvery != 0 {
		return
	}
	r.mu.Lock()
	if len(r.samplesNs) < r.maxSamples {
		r.samplesNs = append(r.samplesNs, d.Nanoseconds())
	}
	r.mu.Unlock()
}

func (r *latencyRecorder) percentileNs(p float64) float64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.samplesNs) == 0 {
		return 0
	}
	sorted := append([]int64(nil), r.samplesNs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	return float64(sorted[idx])
}

func BenchmarkMillionActors(b *testing.B) {
	target := benchmarkTarget()
	b.ReportAllocs()

	rt := actor.NewRuntime()
	handler := func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}
	ids := make([]string, target)

	var before, afterCreate, afterSend gruntime.MemStats
	gruntime.GC()
	gruntime.ReadMemStats(&before)

	for i := 0; i < target; i++ {
		id := fmt.Sprintf("actor-%d", i)
		ids[i] = id
		if _, err := rt.CreateActor(id, 0, handler); err != nil {
			b.Fatalf("create actor %d: %v", i, err)
		}
	}

	gruntime.GC()
	gruntime.ReadMemStats(&afterCreate)
	actorHeapBytes := int64(afterCreate.HeapAlloc) - int64(before.HeapAlloc)
	if actorHeapBytes > 0 {
		b.ReportMetric(float64(actorHeapBytes)/float64(target), "heapB/actor")
	}
	b.ReportMetric(float64(target), "actors")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := ids[i%target]
		ack := rt.Send(context.Background(), id, i)
		if ack.Result != actor.SubmitAccepted {
			b.Fatalf("send result: %s", ack.Result)
		}
	}
	b.StopTimer()

	gruntime.GC()
	gruntime.ReadMemStats(&afterSend)
	sendHeapBytes := int64(afterSend.HeapAlloc) - int64(afterCreate.HeapAlloc)
	if sendHeapBytes > 0 {
		b.ReportMetric(float64(sendHeapBytes)/float64(b.N), "heapB/send")
	}
}

func BenchmarkMillionActorsParallelSend(b *testing.B) {
	target := benchmarkTarget()
	b.ReportAllocs()

	rt := actor.NewRuntime()
	handler := func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}
	ids := make([]string, target)
	for i := 0; i < target; i++ {
		id := fmt.Sprintf("actor-%d", i)
		ids[i] = id
		if _, err := rt.CreateActor(id, 0, handler); err != nil {
			b.Fatalf("create actor %d: %v", i, err)
		}
	}

	var seq atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := seq.Add(1) - 1
			id := ids[i%uint64(target)]
			ack := rt.Send(context.Background(), id, i)
			if ack.Result != actor.SubmitAccepted {
				b.Fatalf("send result: %s", ack.Result)
			}
		}
	})
}

func BenchmarkMillionActorsParallelLatency(b *testing.B) {
	target := benchmarkTarget()
	sampleEvery := uint64(envInt("WC_LAT_SAMPLE_EVERY", 64))
	maxSamples := envInt("WC_LAT_MAX_SAMPLES", 200000)
	latency := newLatencyRecorder(sampleEvery, maxSamples)

	rt := actor.NewRuntime(actor.WithMetrics(latency))
	handler := func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}
	ids := make([]string, target)
	for i := 0; i < target; i++ {
		id := fmt.Sprintf("actor-%d", i)
		ids[i] = id
		if _, err := rt.CreateActor(id, 0, handler); err != nil {
			b.Fatalf("create actor %d: %v", i, err)
		}
	}

	var seq atomic.Uint64
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := seq.Add(1) - 1
			id := ids[i%uint64(target)]
			ack := rt.Send(context.Background(), id, i)
			if ack.Result != actor.SubmitAccepted {
				b.Fatalf("send result: %s", ack.Result)
			}
		}
	})
	b.StopTimer()

	b.ReportMetric(float64(latency.seen.Load()), "enqueue_obs")
	b.ReportMetric(latency.percentileNs(0.50), "p50_enqueue_ns")
	b.ReportMetric(latency.percentileNs(0.95), "p95_enqueue_ns")
	b.ReportMetric(latency.percentileNs(0.99), "p99_enqueue_ns")
}
