package metrics

import "time"

// Hooks captures runtime signals without enforcing a concrete metrics backend.
type Hooks interface {
	// ObserveMailboxDepth records the current mailbox depth for an actor.
	ObserveMailboxDepth(actorID string, depth int)
	// ObserveEnqueueLatency records the time spent enqueuing a message.
	ObserveEnqueueLatency(actorID string, d time.Duration)
	// ObserveLocalSendLatency records the end-to-end latency of a local send.
	ObserveLocalSendLatency(actorID string, d time.Duration)
	// ObserveProcessingLatency records the time an actor spent processing a message.
	ObserveProcessingLatency(actorID string, d time.Duration)
	// ObserveLocalRouting records the outcome of a local message routing attempt.
	ObserveLocalRouting(actorID string, outcome string)
	// ObservePanicIntercept records that a panic was intercepted in an actor.
	ObservePanicIntercept(actorID string)
	// ObserveMailboxPreservedDepth records the mailbox depth preserved across a restart.
	ObserveMailboxPreservedDepth(actorID string, depth int)
	// ObserveRestart records that an actor was restarted.
	ObserveRestart(actorID string)
	// ObservePIDLookupLatency records the latency of a PID lookup.
	ObservePIDLookupLatency(pidKey string, d time.Duration)
	// ObserveRegistryLookupLatency records the latency of a registry name lookup.
	ObserveRegistryLookupLatency(name string, d time.Duration)
	// ObserveRegistryOperation records the outcome of a registry operation.
	ObserveRegistryOperation(result string)
	// ObserveLifecycleHook records the result of a lifecycle hook invocation.
	ObserveLifecycleHook(phase string, result string)
	// ObserveGuardrailDecision records the result of a guardrail evaluation.
	ObserveGuardrailDecision(scope string, result string)
	// ObserveAskOutcome records the outcome of an ask (request-reply) call.
	ObserveAskOutcome(outcome string)
	// ObserveRouterOutcome records the outcome of a router dispatch.
	ObserveRouterOutcome(strategy string, outcome string)
	// ObserveBatchOutcome records the result of a batch processing operation.
	ObserveBatchOutcome(result string)
	// ObservePubSubOutcome records the result of a pub/sub operation.
	ObservePubSubOutcome(operation string, result string, matchedCount int)

	// ObserveRemoteSendLatency records the latency of a remote send to another cluster node.
	ObserveRemoteSendLatency(targetNode string, d time.Duration)
	// ObserveRemoteSendOutcome records the outcome of a remote send attempt.
	ObserveRemoteSendOutcome(targetNode string, outcome string)
	// ObserveClusterMemberEvent records a cluster membership change event.
	ObserveClusterMemberEvent(eventType string)
	// ObserveCodecLatency records the latency of a codec encode or decode operation.
	ObserveCodecLatency(operation string, d time.Duration)
}

// NopHooks is a no-op implementation of Hooks that discards all observations.
type NopHooks struct{}

func (NopHooks) ObserveMailboxDepth(string, int)               {}
func (NopHooks) ObserveEnqueueLatency(string, time.Duration)   {}
func (NopHooks) ObserveLocalSendLatency(string, time.Duration) {}
func (NopHooks) ObserveProcessingLatency(string, time.Duration) {
}
func (NopHooks) ObserveLocalRouting(string, string)                 {}
func (NopHooks) ObservePanicIntercept(string)                       {}
func (NopHooks) ObserveMailboxPreservedDepth(string, int)           {}
func (NopHooks) ObserveRestart(string)                              {}
func (NopHooks) ObservePIDLookupLatency(string, time.Duration)      {}
func (NopHooks) ObserveRegistryLookupLatency(string, time.Duration) {}
func (NopHooks) ObserveRegistryOperation(string)                    {}
func (NopHooks) ObserveLifecycleHook(string, string)                {}
func (NopHooks) ObserveGuardrailDecision(string, string)            {}
func (NopHooks) ObserveAskOutcome(string)                           {}
func (NopHooks) ObserveRouterOutcome(string, string)                {}
func (NopHooks) ObserveBatchOutcome(string)                         {}
func (NopHooks) ObservePubSubOutcome(string, string, int)           {}
func (NopHooks) ObserveRemoteSendLatency(string, time.Duration)     {}
func (NopHooks) ObserveRemoteSendOutcome(string, string)            {}
func (NopHooks) ObserveClusterMemberEvent(string)                   {}
func (NopHooks) ObserveCodecLatency(string, time.Duration)          {}
