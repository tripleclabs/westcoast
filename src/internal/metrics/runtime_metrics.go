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
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{
		pidLookup:     make([]int64, 0, 4096),
		localSendNs:   make([]int64, 0, 4096),
		localByResult: map[string]int64{},
		lifecycleBy:   map[string]int64{},
		guardrailBy:   map[string]int64{},
		askBy:         map[string]int64{},
		routerBy:      map[string]int64{},
	}
}

func (m *RuntimeMetrics) ObserveMailboxDepth(string, int)                {}
func (m *RuntimeMetrics) ObserveEnqueueLatency(string, time.Duration)    {}
func (m *RuntimeMetrics) ObserveProcessingLatency(string, time.Duration) {}
func (m *RuntimeMetrics) ObserveLocalSendLatency(_ string, d time.Duration) {
	m.mu.Lock()
	m.localSendNs = append(m.localSendNs, d.Nanoseconds())
	m.localSent++
	m.mu.Unlock()
}
func (m *RuntimeMetrics) ObserveLocalRouting(_ string, outcome string) {
	m.mu.Lock()
	m.localByResult[outcome]++
	m.mu.Unlock()
}
func (m *RuntimeMetrics) ObservePanicIntercept(string)             {}
func (m *RuntimeMetrics) ObserveMailboxPreservedDepth(string, int) {}
func (m *RuntimeMetrics) ObserveRestart(string)                    {}

func (m *RuntimeMetrics) ObservePIDLookupLatency(_ string, d time.Duration) {
	m.mu.Lock()
	m.pidLookup = append(m.pidLookup, d.Nanoseconds())
	m.mu.Unlock()
}

func (m *RuntimeMetrics) ObserveRegistryLookupLatency(_ string, _ time.Duration) {}
func (m *RuntimeMetrics) ObserveRegistryOperation(_ string)                      {}
func (m *RuntimeMetrics) ObserveLifecycleHook(phase string, result string) {
	m.mu.Lock()
	m.lifecycleBy[phase+":"+result]++
	m.mu.Unlock()
}
func (m *RuntimeMetrics) ObserveGuardrailDecision(scope string, result string) {
	m.mu.Lock()
	m.guardrailBy[scope+":"+result]++
	m.mu.Unlock()
}
func (m *RuntimeMetrics) ObserveAskOutcome(outcome string) {
	m.mu.Lock()
	m.askBy[outcome]++
	m.mu.Unlock()
}
func (m *RuntimeMetrics) ObserveRouterOutcome(strategy string, outcome string) {
	m.mu.Lock()
	m.routerBy[strategy+":"+outcome]++
	m.mu.Unlock()
}

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
