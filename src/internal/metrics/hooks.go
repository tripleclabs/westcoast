package metrics

import "time"

// Hooks captures runtime signals without enforcing a concrete metrics backend.
type Hooks interface {
	ObserveMailboxDepth(actorID string, depth int)
	ObserveEnqueueLatency(actorID string, d time.Duration)
	ObserveLocalSendLatency(actorID string, d time.Duration)
	ObserveProcessingLatency(actorID string, d time.Duration)
	ObserveLocalRouting(actorID string, outcome string)
	ObservePanicIntercept(actorID string)
	ObserveMailboxPreservedDepth(actorID string, depth int)
	ObserveRestart(actorID string)
	ObservePIDLookupLatency(pidKey string, d time.Duration)
}

type NopHooks struct{}

func (NopHooks) ObserveMailboxDepth(string, int)               {}
func (NopHooks) ObserveEnqueueLatency(string, time.Duration)   {}
func (NopHooks) ObserveLocalSendLatency(string, time.Duration) {}
func (NopHooks) ObserveProcessingLatency(string, time.Duration) {
}
func (NopHooks) ObserveLocalRouting(string, string)            {}
func (NopHooks) ObservePanicIntercept(string)                  {}
func (NopHooks) ObserveMailboxPreservedDepth(string, int)      {}
func (NopHooks) ObserveRestart(string)                         {}
func (NopHooks) ObservePIDLookupLatency(string, time.Duration) {}
