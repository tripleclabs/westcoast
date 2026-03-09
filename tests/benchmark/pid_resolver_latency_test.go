package benchmark_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

func pidBenchTarget() int {
	if v := os.Getenv("WC_BENCH_TARGET"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 200000
}

type pidLookupMetrics struct {
	mu      sync.Mutex
	samples []int64
	seen    atomic.Uint64
}

func (m *pidLookupMetrics) ObserveMailboxDepth(string, int)                {}
func (m *pidLookupMetrics) ObserveEnqueueLatency(string, time.Duration)    {}
func (m *pidLookupMetrics) ObserveLocalSendLatency(string, time.Duration)  {}
func (m *pidLookupMetrics) ObserveProcessingLatency(string, time.Duration) {}
func (m *pidLookupMetrics) ObserveLocalRouting(string, string)             {}
func (m *pidLookupMetrics) ObserveRestart(string)                          {}
func (m *pidLookupMetrics) ObservePIDLookupLatency(_ string, d time.Duration) {
	n := m.seen.Add(1)
	if n != 1 && n%64 != 0 {
		return
	}
	m.mu.Lock()
	m.samples = append(m.samples, d.Nanoseconds())
	m.mu.Unlock()
}

func (m *pidLookupMetrics) p95() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.samples) == 0 {
		return 0
	}
	arr := append([]int64(nil), m.samples...)
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	idx := int(float64(len(arr)-1) * 0.95)
	return time.Duration(arr[idx]) * time.Nanosecond
}

func BenchmarkPIDResolverLatency(b *testing.B) {
	target := pidBenchTarget()
	m := &pidLookupMetrics{}
	rt := actor.NewRuntime(actor.WithMetrics(m))
	h := func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil }
	pids := make([]actor.PID, target)
	for i := 0; i < target; i++ {
		id := fmt.Sprintf("pidbench-%d", i)
		ref, err := rt.CreateActor(id, 0, h)
		if err != nil {
			b.Fatal(err)
		}
		pid, err := ref.PID("bench")
		if err != nil {
			b.Fatal(err)
		}
		pids[i] = pid
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ack := rt.SendPID(context.Background(), pids[i%target], i)
		if ack.Outcome != actor.PIDDelivered {
			b.Fatalf("outcome=%s", ack.Outcome)
		}
	}
	b.StopTimer()
	p95 := m.p95()
	b.ReportMetric(float64(p95.Nanoseconds()), "p95_pid_lookup_ns")
	if p95 > 25*time.Microsecond {
		b.Fatalf("p95 pid lookup too high: %s", p95)
	}
}
