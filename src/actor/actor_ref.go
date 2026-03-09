package actor

import "context"

type SubmitAck struct {
	Result    SubmitResult
	MessageID uint64
}

type ActorRef struct {
	runtime *Runtime
	actorID string
}

func (a *ActorRef) ID() string { return a.actorID }

func (a *ActorRef) Send(ctx context.Context, payload any) SubmitAck {
	return a.runtime.Send(ctx, a.actorID, payload)
}

func (a *ActorRef) Stop() StopResult {
	return a.runtime.Stop(a.actorID)
}

func (a *ActorRef) Status() ActorStatus {
	return a.runtime.Status(a.actorID)
}

func (a *ActorRef) SetState(any) error {
	return ErrStateMutationForbidden
}

func (a *ActorRef) PID(namespace string) (PID, error) {
	return a.runtime.IssuePID(namespace, a.actorID)
}

func (a *ActorRef) SendPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	return a.runtime.SendPID(ctx, pid, payload)
}

func (a *ActorRef) RegisterTypeRoute(typeName, schemaVersion, handlerKey string) error {
	return a.runtime.RegisterTypeRoute(a.actorID, typeName, schemaVersion, handlerKey)
}

func (a *ActorRef) RegisterFallbackRoute(handlerKey string) error {
	return a.runtime.RegisterFallbackRoute(a.actorID, handlerKey)
}

func (a *ActorRef) RegisterName(name string) (RegistryRegisterAck, error) {
	return a.runtime.RegisterName(a.actorID, name, "default")
}

func (a *ActorRef) LifecycleOutcomes() []LifecycleHookOutcome {
	return a.runtime.LifecycleOutcomes(a.actorID)
}

func (a *ActorRef) CrossSendActorID(ctx context.Context, targetActorID string, payload any) SubmitAck {
	return a.runtime.CrossActorSendByActorID(ctx, a.actorID, targetActorID, payload)
}

func (a *ActorRef) CrossSendPID(ctx context.Context, pid PID, payload any) PIDSendAck {
	return a.runtime.CrossActorSendPID(ctx, a.actorID, pid, payload)
}

func (a *ActorRef) GuardrailOutcomes() []GuardrailOutcome {
	return a.runtime.GuardrailOutcomes(a.actorID)
}
