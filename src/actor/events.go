package actor

import (
	"sync"
	"time"
)

type EventType string

const (
	EventActorStarted          EventType = "actor_started"
	EventActorStopped          EventType = "actor_stopped"
	EventActorFailed           EventType = "actor_failed"
	EventActorRestarted        EventType = "actor_restarted"
	EventActorEscalated        EventType = "actor_escalated"
	EventMessageRoutedExact    EventType = "message_routed_exact"
	EventMessageRoutedFallback EventType = "message_routed_fallback"
	EventMessageProcessed      EventType = "message_processed"
	EventMessageRejected       EventType = "message_rejected"
	EventPIDResolved           EventType = "pid_resolved"
	EventPIDUnresolved         EventType = "pid_unresolved"
	EventPIDRejected           EventType = "pid_rejected"
	EventPIDDelivered          EventType = "pid_delivered"
	EventRegistryRegister      EventType = "registry_register"
	EventRegistryLookup        EventType = "registry_lookup"
	EventRegistryUnregister    EventType = "registry_unregister"
	EventLifecycleHook         EventType = "lifecycle_hook"
	EventGuardrailDecision     EventType = "guardrail_decision"
	EventReadinessValidation   EventType = "readiness_validation"
	EventAskLifecycle          EventType = "ask_lifecycle"
	EventRouterLifecycle       EventType = "router_lifecycle"
	EventBatchLifecycle        EventType = "batch_lifecycle"
)

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

type EventEmitter interface {
	Emit(Event)
}

type NopEmitter struct{}

func (NopEmitter) Emit(Event) {}

type MemoryEmitter struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryEmitter() *MemoryEmitter { return &MemoryEmitter{} }

func (m *MemoryEmitter) Emit(e Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
}

func (m *MemoryEmitter) Events() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}
