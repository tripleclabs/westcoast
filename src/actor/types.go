package actor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var (
	ErrDuplicateActorID       = errors.New("duplicate_actor_id")
	ErrActorNotFound          = errors.New("actor_not_found")
	ErrActorStopped           = errors.New("actor_stopped")
	ErrStateMutationForbidden = errors.New("state_mutation_forbidden")
	ErrRegistryNameInvalid    = errors.New("registry_name_invalid")
	ErrRegistryDuplicateName  = errors.New("registry_duplicate_name")
	ErrRegistryNameNotFound   = errors.New("registry_name_not_found")
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
	MessageID            uint64
	ActorID              string
	Result               ProcessingResult
	SupervisionDecision  SupervisionDecision
	SupervisionIteration int
	CompletedAt          time.Time
	ErrorCode            string
}

type Handler func(ctx context.Context, state any, msg Message) (nextState any, err error)

type SupervisionDecision string

const (
	DecisionRestart  SupervisionDecision = "restart"
	DecisionStop     SupervisionDecision = "stop"
	DecisionEscalate SupervisionDecision = "escalate"
)

type RegistryOperationResult string

const (
	RegistryRegisterSuccess         RegistryOperationResult = "register_success"
	RegistryRegisterRejectedDup     RegistryOperationResult = "register_rejected_duplicate"
	RegistryLookupHit               RegistryOperationResult = "lookup_hit"
	RegistryLookupNotFound          RegistryOperationResult = "lookup_not_found"
	RegistryUnregisterSuccess       RegistryOperationResult = "unregister_success"
	RegistryUnregisterLifecycleTerm RegistryOperationResult = "unregister_lifecycle_terminal"
)

type RegistryRegisterAck struct {
	Result RegistryOperationResult
	Name   string
	PID    PID
}

type RegistryLookupAck struct {
	Result RegistryOperationResult
	Name   string
	PID    PID
}

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validateRegistryName(name string) error {
	if !registryNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrRegistryNameInvalid, name)
	}
	return nil
}
