package metrics

import (
	"sort"
	"sync"
	"time"
)

// RuntimeMetrics is an in-memory implementation used for tests/benchmarks.
type RuntimeMetrics struct {
	mu            sync.Mutex
	pidLookup     []int64
	localSendNs   []int64
	localSent     int64
	localByResult map[string]int64
	lifecycleBy   map[string]int64
	guardrailBy   map[string]int64
	askBy         map[string]int64
	routerBy      map[string]int64
	batchBy       map[string]int64
	pubsubBy      map[string]int64
}

// NewRuntimeMetrics creates a RuntimeMetrics with pre-allocated internal storage.
func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{
		pidLookup:     make([]int64, 0, 4096),
		localSendNs:   make([]int64, 0, 4096),
		localByResult: map[string]int64{},
		lifecycleBy:   map[string]int64{},
		guardrailBy:   map[string]int64{},
		askBy:         map[string]int64{},
		routerBy:      map[string]int64{},
		batchBy:       map[string]int64{},
		pubsubBy:      map[string]int64{},
	}
}

// ObserveMailboxDepth implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveMailboxDepth(string, int) {}

// ObserveEnqueueLatency implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveEnqueueLatency(string, time.Duration) {}

// ObserveProcessingLatency implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveProcessingLatency(string, time.Duration) {}

// ObserveLocalSendLatency records a local send latency sample in nanoseconds.
func (m *RuntimeMetrics) ObserveLocalSendLatency(_ string, d time.Duration) {
	m.mu.Lock()
	m.localSendNs = append(m.localSendNs, d.Nanoseconds())
	m.localSent++
	m.mu.Unlock()
}

// ObserveLocalRouting increments the counter for the given routing outcome.
func (m *RuntimeMetrics) ObserveLocalRouting(_ string, outcome string) {
	m.mu.Lock()
	m.localByResult[outcome]++
	m.mu.Unlock()
}

// ObservePanicIntercept implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObservePanicIntercept(string) {}

// ObserveMailboxPreservedDepth implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveMailboxPreservedDepth(string, int) {}

// ObserveRestart implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveRestart(string) {}

// ObservePIDLookupLatency records a PID lookup latency sample in nanoseconds.
func (m *RuntimeMetrics) ObservePIDLookupLatency(_ string, d time.Duration) {
	m.mu.Lock()
	m.pidLookup = append(m.pidLookup, d.Nanoseconds())
	m.mu.Unlock()
}

// ObserveRegistryLookupLatency implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveRegistryLookupLatency(_ string, _ time.Duration) {}

// ObserveRegistryOperation implements Hooks and is a no-op in RuntimeMetrics.
func (m *RuntimeMetrics) ObserveRegistryOperation(_ string) {}

// ObserveLifecycleHook increments the counter for the given phase and result combination.
func (m *RuntimeMetrics) ObserveLifecycleHook(phase string, result string) {
	m.mu.Lock()
	m.lifecycleBy[phase+":"+result]++
	m.mu.Unlock()
}

// ObserveGuardrailDecision increments the counter for the given scope and result combination.
func (m *RuntimeMetrics) ObserveGuardrailDecision(scope string, result string) {
	m.mu.Lock()
	m.guardrailBy[scope+":"+result]++
	m.mu.Unlock()
}

// ObserveAskOutcome increments the counter for the given ask outcome.
func (m *RuntimeMetrics) ObserveAskOutcome(outcome string) {
	m.mu.Lock()
	m.askBy[outcome]++
	m.mu.Unlock()
}

// ObserveRouterOutcome increments the counter for the given strategy and outcome combination.
func (m *RuntimeMetrics) ObserveRouterOutcome(strategy string, outcome string) {
	m.mu.Lock()
	m.routerBy[strategy+":"+outcome]++
	m.mu.Unlock()
}

// ObserveBatchOutcome increments the counter for the given batch result.
func (m *RuntimeMetrics) ObserveBatchOutcome(result string) {
	m.mu.Lock()
	m.batchBy[result]++
	m.mu.Unlock()
}

// ObservePubSubOutcome increments the counter for the given pub/sub operation and result.
func (m *RuntimeMetrics) ObservePubSubOutcome(operation string, result string, _ int) {
	m.mu.Lock()
	m.pubsubBy[operation+":"+result]++
	m.mu.Unlock()
}

// PIDLookupP95 returns the 95th percentile PID lookup latency.
func (m *RuntimeMetrics) PIDLookupP95() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.pidLookup) == 0 {
		return 0
	}
	arr := append([]int64(nil), m.pidLookup...)
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	idx := int(float64(len(arr)-1) * 0.95)
	return time.Duration(arr[idx]) * time.Nanosecond
}

// LocalSendP95 returns the 95th percentile local send latency.
func (m *RuntimeMetrics) LocalSendP95() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.localSendNs) == 0 {
		return 0
	}
	arr := append([]int64(nil), m.localSendNs...)
	sort.Slice(arr, func(i, j int) bool { return arr[i] < arr[j] })
	idx := int(float64(len(arr)-1) * 0.95)
	return time.Duration(arr[idx]) * time.Nanosecond
}
