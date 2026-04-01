package actor

import (
	"sync"
	"time"
)

// EventType identifies the kind of event emitted by the runtime.
type EventType string

const (
	// EventActorStarted is emitted when an actor transitions to the running state.
	EventActorStarted EventType = "actor_started"
	// EventActorStopped is emitted when an actor terminates.
	EventActorStopped EventType = "actor_stopped"
	// EventActorFailed is emitted when a handler returns an error or panics.
	EventActorFailed EventType = "actor_failed"
	// EventActorRestarted is emitted when an actor is restarted by the supervisor.
	EventActorRestarted EventType = "actor_restarted"
	// EventActorEscalated is emitted when a supervisor escalates a failure.
	EventActorEscalated EventType = "actor_escalated"
	// EventMessageRoutedExact is emitted when a message matches an exact type route.
	EventMessageRoutedExact EventType = "message_routed_exact"
	// EventMessageRoutedFallback is emitted when a message is handled by the fallback route.
	EventMessageRoutedFallback EventType = "message_routed_fallback"
	// EventMessageProcessed is emitted after a message is successfully processed.
	EventMessageProcessed EventType = "message_processed"
	// EventMessageRejected is emitted when a message is rejected before processing.
	EventMessageRejected EventType = "message_rejected"
	// EventPIDResolved is emitted when a PID is successfully resolved.
	EventPIDResolved EventType = "pid_resolved"
	// EventPIDUnresolved is emitted when a PID cannot be resolved.
	EventPIDUnresolved EventType = "pid_unresolved"
	// EventPIDRejected is emitted when a PID-addressed send is rejected.
	EventPIDRejected EventType = "pid_rejected"
	// EventPIDDelivered is emitted when a PID-addressed message is delivered.
	EventPIDDelivered EventType = "pid_delivered"
	// EventRegistryRegister is emitted when a name registration is attempted.
	EventRegistryRegister EventType = "registry_register"
	// EventRegistryLookup is emitted when a name lookup is performed.
	EventRegistryLookup EventType = "registry_lookup"
	// EventRegistryUnregister is emitted when a name is unregistered.
	EventRegistryUnregister EventType = "registry_unregister"
	// EventLifecycleHook is emitted when a lifecycle hook completes.
	EventLifecycleHook EventType = "lifecycle_hook"
	// EventGuardrailDecision is emitted when a guardrail policy is evaluated.
	EventGuardrailDecision EventType = "guardrail_decision"
	// EventReadinessValidation is emitted when a readiness check is performed.
	EventReadinessValidation EventType = "readiness_validation"
	// EventAskLifecycle is emitted for ask request-response lifecycle transitions.
	EventAskLifecycle EventType = "ask_lifecycle"
	// EventRouterLifecycle is emitted for router dispatch outcomes.
	EventRouterLifecycle EventType = "router_lifecycle"
	// EventBatchLifecycle is emitted for batch processing outcomes.
	EventBatchLifecycle EventType = "batch_lifecycle"
	// EventBrokerLifecycle is emitted for pubsub broker operations.
	EventBrokerLifecycle EventType = "broker_lifecycle"
)

// Event is a structured record emitted by the runtime for observability.
type Event struct {
	EventID             uint64
	Type                EventType
	ActorID             string
	MessageID           uint64
	PIDNamespace        string
	PIDActorID          string
	PIDGeneration       uint64
	TypeName            string
	SchemaVersion       string
	SupervisionDecision string
	RestartCount        int
	RegistryName        string
	LifecyclePhase      string
	GatewayMode         string
	ReadinessScope      string
	RequestID           string
	ReplyToNamespace    string
	ReplyToActorID      string
	ReplyToGeneration   uint64
	RouterStrategy      string
	SelectedWorker      string
	BatchSize           int
	Timestamp           time.Time
	Result              string
	ErrorCode           string
}

// EventEmitter receives events emitted by the runtime.
type EventEmitter interface {
	Emit(Event)
}

// NopEmitter is an EventEmitter that discards all events.
type NopEmitter struct{}

// Emit discards the event.
func (NopEmitter) Emit(Event) {}

// MemoryEmitter is an EventEmitter that stores events in memory for testing.
type MemoryEmitter struct {
	mu     sync.Mutex
	events []Event
}

// NewMemoryEmitter creates a new MemoryEmitter.
func NewMemoryEmitter() *MemoryEmitter { return &MemoryEmitter{} }

// Emit appends the event to the in-memory store.
func (m *MemoryEmitter) Emit(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

// Events returns a copy of all emitted events.
func (m *MemoryEmitter) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}
