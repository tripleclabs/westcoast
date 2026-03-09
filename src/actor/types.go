package actor

import (
	"context"
	"errors"
	"time"
)

var (
	ErrDuplicateActorID       = errors.New("duplicate_actor_id")
	ErrActorNotFound          = errors.New("actor_not_found")
	ErrActorStopped           = errors.New("actor_stopped")
	ErrStateMutationForbidden = errors.New("state_mutation_forbidden")
)

type ActorStatus string

const (
	ActorStarting   ActorStatus = "starting"
	ActorRunning    ActorStatus = "running"
	ActorRestarting ActorStatus = "restarting"
	ActorStopped    ActorStatus = "stopped"
)

type SubmitResult string

const (
	SubmitAccepted                SubmitResult = "accepted"
	SubmitRejectedFull            SubmitResult = "rejected_full"
	SubmitRejectedStop            SubmitResult = "rejected_stopped"
	SubmitRejectedFound           SubmitResult = "rejected_not_found"
	SubmitRejectedUnsupportedType SubmitResult = "rejected_unsupported_type"
	SubmitRejectedNilPayload      SubmitResult = "rejected_nil_payload"
	SubmitRejectedVersionMismatch SubmitResult = "rejected_version_mismatch"
)

type StopResult string

const (
	StopStopped  StopResult = "stopped"
	StopAlready  StopResult = "already_stopped"
	StopNotFound StopResult = "not_found"
)

type Message struct {
	ID            uint64
	ActorID       string
	SenderActorID string
	Payload       any
	TypeName      string
	SchemaVersion string
	AcceptedAt    time.Time
	Attempt       int
}

type ProcessingResult string

const (
	ResultSuccess                 ProcessingResult = "success"
	ResultDelivered               ProcessingResult = "delivered"
	ResultFailed                  ProcessingResult = "failed"
	ResultRejectedFull            ProcessingResult = "rejected_full"
	ResultRejectedStop            ProcessingResult = "rejected_stopped"
	ResultRejectedFound           ProcessingResult = "rejected_not_found"
	ResultRejectedUnsupportedType ProcessingResult = "rejected_unsupported_type"
	ResultRejectedNilPayload      ProcessingResult = "rejected_nil_payload"
	ResultRejectedVersionMismatch ProcessingResult = "rejected_version_mismatch"
)

type ProcessingOutcome struct {
	MessageID   uint64
	ActorID     string
	Result      ProcessingResult
	CompletedAt time.Time
	ErrorCode   string
}

type Handler func(ctx context.Context, state any, msg Message) (nextState any, err error)

type SupervisionDecision string

const (
	DecisionRestart  SupervisionDecision = "restart"
	DecisionStop     SupervisionDecision = "stop"
	DecisionEscalate SupervisionDecision = "escalate"
)
