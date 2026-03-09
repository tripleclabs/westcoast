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
