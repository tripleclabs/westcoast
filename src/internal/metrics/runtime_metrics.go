package metrics

import (
	"sort"
	"sync"
	"time"
)

// RuntimeMetrics is an in-memory implementation used for tests/benchmarks.
type RuntimeMetrics struct {
	mu        sync.Mutex
	pidLookup []int64
}

func NewRuntimeMetrics() *RuntimeMetrics {
	return &RuntimeMetrics{pidLookup: make([]int64, 0, 4096)}
}

func (m *RuntimeMetrics) ObserveMailboxDepth(string, int)                {}
func (m *RuntimeMetrics) ObserveEnqueueLatency(string, time.Duration)    {}
func (m *RuntimeMetrics) ObserveProcessingLatency(string, time.Duration) {}
func (m *RuntimeMetrics) ObserveRestart(string)                          {}

func (m *RuntimeMetrics) ObservePIDLookupLatency(_ string, d time.Duration) {
	m.mu.Lock()
	m.pidLookup = append(m.pidLookup, d.Nanoseconds())
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
