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

	"github.com/tripleclabs/westcoast/src/actor"
)

type localPerfPayload struct{ N int }

type localPerfMetrics struct {
	seen    atomic.Uint64
	mu      sync.Mutex
	samples []int64
}

func (m *localPerfMetrics) ObserveMailboxDepth(string, int)                    {}
func (m *localPerfMetrics) ObserveEnqueueLatency(string, time.Duration)        {}
func (m *localPerfMetrics) ObserveProcessingLatency(string, time.Duration)     {}
func (m *localPerfMetrics) ObserveLocalRouting(string, string)                 {}
func (m *localPerfMetrics) ObservePanicIntercept(string)                       {}
func (m *localPerfMetrics) ObserveMailboxPreservedDepth(string, int)           {}
func (m *localPerfMetrics) ObserveRestart(string)                              {}
func (m *localPerfMetrics) ObservePIDLookupLatency(string, time.Duration)      {}
func (m *localPerfMetrics) ObserveRegistryLookupLatency(string, time.Duration) {}
func (m *localPerfMetrics) ObserveRegistryOperation(string)                    {}
func (m *localPerfMetrics) ObserveLifecycleHook(string, string)                {}
func (m *localPerfMetrics) ObserveGuardrailDecision(string, string)            {}
func (m *localPerfMetrics) ObserveAskOutcome(string)                           {}
func (m *localPerfMetrics) ObserveRouterOutcome(string, string)                {}
func (m *localPerfMetrics) ObserveBatchOutcome(string)                         {}
func (m *localPerfMetrics) ObservePubSubOutcome(string, string, int)           {}
func (m *localPerfMetrics) ObserveRemoteSendLatency(string, time.Duration)     {}
func (m *localPerfMetrics) ObserveRemoteSendOutcome(string, string)            {}
func (m *localPerfMetrics) ObserveClusterMemberEvent(string)                   {}
func (m *localPerfMetrics) ObserveCodecLatency(string, time.Duration)          {}
func (m *localPerfMetrics) ObserveLocalSendLatency(_ string, d time.Duration) {
	n := m.seen.Add(1)
	if n != 1 && n%64 != 0 {
		return
	}
	m.mu.Lock()
	m.samples = append(m.samples, d.Nanoseconds())
	m.mu.Unlock()
}

func (m *localPerfMetrics) p95() time.Duration {
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

func localBenchTarget() int {
	if raw := os.Getenv("WC_BENCH_TARGET"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			return n
		}
	}
	return 200000
}

func BenchmarkLocalMessagingPerformance(b *testing.B) {
	target := localBenchTarget()
	metrics := &localPerfMetrics{}
	rt := actor.NewRuntime(actor.WithMetrics(metrics))
	handler := func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil }

	ids := make([]string, target)
	for i := 0; i < target; i++ {
		id := fmt.Sprintf("local-perf-%d", i)
		ids[i] = id
		ref, err := rt.CreateActor(id, nil, handler)
		if err != nil {
			b.Fatalf("create actor: %v", err)
		}
		_ = ref
	}

	b.ReportAllocs()
	b.ResetTimer()
	var seq atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := seq.Add(1) - 1
			id := ids[i%uint64(target)]
			ack := rt.Send(context.Background(), id, localPerfPayload{N: int(i)})
			if ack.Result != actor.SubmitAccepted {
				b.Fatalf("send result=%s", ack.Result)
			}
		}
	})
	b.StopTimer()
	elapsed := b.Elapsed()

	throughput := float64(b.N) / elapsed.Seconds()
	p95 := metrics.p95()
	b.ReportMetric(float64(p95.Nanoseconds()), "p95_local_send_ns")
	b.ReportMetric(throughput, "local_msgs_per_sec")

	if os.Getenv("WC_ENFORCE_LOCAL_PERF_GATE") == "1" {
		if p95 > 25*time.Microsecond {
			b.Fatalf("p95 too high: %s", p95)
		}
		if throughput < 1_000_000 {
			b.Fatalf("throughput too low: %.0f msg/s", throughput)
		}
	}
}
