package metrics

import "time"

// Hooks captures runtime signals without enforcing a concrete metrics backend.
type Hooks interface {
	ObserveMailboxDepth(actorID string, depth int)
	ObserveEnqueueLatency(actorID string, d time.Duration)
	ObserveProcessingLatency(actorID string, d time.Duration)
	ObserveRestart(actorID string)
}

type NopHooks struct{}

func (NopHooks) ObserveMailboxDepth(string, int)             {}
func (NopHooks) ObserveEnqueueLatency(string, time.Duration) {}
func (NopHooks) ObserveProcessingLatency(string, time.Duration) {
}
func (NopHooks) ObserveRestart(string) {}
