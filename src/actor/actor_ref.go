package actor

import (
	"context"
	"time"
)

// SubmitAck is the acknowledgment returned after submitting a message to an actor.
type SubmitAck struct {
	Result    SubmitResult
	MessageID uint64
}

// ActorRef is a handle to a running actor, providing methods to send messages,
// query status, and manage the actor's lifecycle.
type ActorRef struct {
	runtime *Runtime
	actorID string
}

// ID returns the actor's unique identifier.
func (a *ActorRef) ID() string { return a.actorID }

// Send delivers a message to the actor's mailbox.
func (a *ActorRef) Send(ctx context.Context, payload any) SubmitAck {
	return a.runtime.Send(ctx, a.actorID, payload)
}

// Stop terminates the actor, running its stop hook if configured.
func (a *ActorRef) Stop() StopResult {
	return a.runtime.Stop(a.actorID)
}

// Status returns the actor's current lifecycle status.
func (a *ActorRef) Status() ActorStatus {
	return a.runtime.Status(a.actorID)
}

// SetState always returns ErrStateMutationForbidden; actor state cannot be mutated externally.
func (a *ActorRef) SetState(any) error {
	return ErrStateMutationForbidden
}

// PID issues a PID for this actor in the given namespace.
func (a *ActorRef) PID(namespace string) (PID, error) {
	return a.runtime.IssuePID(namespace, a.actorID)
}

// SendPID sends a message to an actor addressed by PID.
func (a *ActorRef) SendPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	return a.runtime.SendPID(ctx, pid, payload)
}

// RegisterTypeRoute registers an exact type-based message routing rule for this actor.
func (a *ActorRef) RegisterTypeRoute(typeName, schemaVersion, handlerKey string) error {
	return a.runtime.RegisterTypeRoute(a.actorID, typeName, schemaVersion, handlerKey)
}

// RegisterFallbackRoute registers a fallback routing rule for unmatched message types.
func (a *ActorRef) RegisterFallbackRoute(handlerKey string) error {
	return a.runtime.RegisterFallbackRoute(a.actorID, handlerKey)
}

// RegisterName registers a human-readable name for this actor in the default namespace.
func (a *ActorRef) RegisterName(name string) (RegistryRegisterAck, error) {
	return a.runtime.RegisterName(a.actorID, name, "default")
}

// LifecycleOutcomes returns the lifecycle hook outcomes recorded for this actor.
func (a *ActorRef) LifecycleOutcomes() []LifecycleHookOutcome {
	return a.runtime.LifecycleOutcomes(a.actorID)
}

// CrossSendActorID sends a message to another actor by actor ID, subject to PID interaction policy.
func (a *ActorRef) CrossSendActorID(ctx context.Context, targetActorID string, payload any) SubmitAck {
	return a.runtime.CrossActorSendByActorID(ctx, a.actorID, targetActorID, payload)
}

// CrossSendPID sends a message to another actor by PID.
func (a *ActorRef) CrossSendPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	return a.runtime.CrossActorSendPID(ctx, a.actorID, pid, payload)
}

// GuardrailOutcomes returns the guardrail policy outcomes recorded for this actor.
func (a *ActorRef) GuardrailOutcomes() []GuardrailOutcome {
	return a.runtime.GuardrailOutcomes(a.actorID)
}

// Ask sends a request and waits for a reply within the given timeout.
func (a *ActorRef) Ask(ctx context.Context, payload any, timeout time.Duration) (AskResult, error) {
	return a.runtime.Ask(ctx, a.actorID, payload, timeout)
}

// AskOutcomes returns the ask interaction outcomes recorded for this actor.
func (a *ActorRef) AskOutcomes() []AskOutcome {
	return a.runtime.AskOutcomes(a.actorID)
}

// ConfigureRouter sets up this actor as a router with the given strategy and worker pool.
func (a *ActorRef) ConfigureRouter(strategy RouterStrategy, workers []string) error {
	return a.runtime.ConfigureRouter(a.actorID, strategy, workers)
}

// Route sends a message through this actor's router to a selected worker.
func (a *ActorRef) Route(ctx context.Context, payload any) SubmitAck {
	return a.runtime.Route(ctx, a.actorID, payload)
}

// RoutingOutcomes returns the routing outcomes recorded for this actor.
func (a *ActorRef) RoutingOutcomes() []RoutingOutcome {
	return a.runtime.RoutingOutcomes(a.actorID)
}

// ConfigureBatching enables batch processing for this actor with the given max batch size.
func (a *ActorRef) ConfigureBatching(maxBatchSize int, receiver BatchReceive) error {
	return a.runtime.ConfigureBatching(a.actorID, maxBatchSize, receiver)
}

// DisableBatching turns off batch processing for this actor.
func (a *ActorRef) DisableBatching() error {
	return a.runtime.DisableBatching(a.actorID)
}

// BatchOutcomes returns the batch processing outcomes recorded for this actor.
func (a *ActorRef) BatchOutcomes() []BatchOutcome {
	return a.runtime.BatchOutcomes(a.actorID)
}

// BrokerOutcomes returns the pubsub broker outcomes recorded for this actor.
func (a *ActorRef) BrokerOutcomes() []BrokerOutcome {
	return a.runtime.BrokerOutcomes(a.actorID)
}

// BrokerPublishedCount returns the number of messages published through this broker actor.
func (a *ActorRef) BrokerPublishedCount() int {
	return a.runtime.BrokerPublishedCount(a.actorID)
}
