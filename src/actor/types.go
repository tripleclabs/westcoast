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
	ErrLifecycleStartFailed   = errors.New("lifecycle_start_failed")
	ErrLifecycleStopFailed    = errors.New("lifecycle_stop_failed")
	ErrNonPIDCrossActor       = errors.New("non_pid_cross_actor_rejected")
	ErrGatewayRouteFailed     = errors.New("gateway_route_failed")
	ErrAskTimeout             = errors.New("ask_timeout")
	ErrAskCanceled            = errors.New("ask_canceled")
	ErrAskInvalidTimeout      = errors.New("ask_invalid_timeout")
	ErrAskReplyTargetInvalid  = errors.New("ask_reply_target_invalid")
	ErrBatchConfigInvalid     = errors.New("batch_config_invalid")
)

type ActorStatus string

const (
	ActorStarting   ActorStatus = "starting"
	ActorRunning    ActorStatus = "running"
	ActorRestarting ActorStatus = "restarting"
	ActorStopping   ActorStatus = "stopping"
	ActorStopped    ActorStatus = "stopped"
)

type LifecycleHookPhase string

const (
	LifecyclePhaseStart LifecycleHookPhase = "start"
	LifecyclePhaseStop  LifecycleHookPhase = "stop"
)

type LifecycleHookResult string

const (
	LifecycleStartSuccess LifecycleHookResult = "start_success"
	LifecycleStartFailed  LifecycleHookResult = "start_failed"
	LifecycleStopSuccess  LifecycleHookResult = "stop_success"
	LifecycleStopFailed   LifecycleHookResult = "stop_failed"
)

type LifecycleHook func(ctx context.Context, actorID string) error

type LifecycleHookOutcome struct {
	ActorID     string
	Phase       LifecycleHookPhase
	Result      LifecycleHookResult
	CompletedAt time.Time
	ErrorCode   string
}

type PIDInteractionPolicyMode string

const (
	PIDInteractionPolicyDisabled PIDInteractionPolicyMode = "disabled"
	PIDInteractionPolicyPIDOnly  PIDInteractionPolicyMode = "pid_only"
)

type GatewayRouteMode string

const (
	GatewayRouteLocalDirect     GatewayRouteMode = "local_direct"
	GatewayRouteGatewayMediated GatewayRouteMode = "gateway_mediated"
)

type GuardrailOutcomeType string

const (
	GuardrailPolicyAccept        GuardrailOutcomeType = "policy_accept"
	GuardrailPolicyRejectNonPID  GuardrailOutcomeType = "policy_reject_non_pid"
	GuardrailGatewayRouteSuccess GuardrailOutcomeType = "gateway_route_success"
	GuardrailGatewayRouteFailure GuardrailOutcomeType = "gateway_route_failure"
)

type GuardrailOutcome struct {
	ActorID     string
	PolicyMode  PIDInteractionPolicyMode
	GatewayMode GatewayRouteMode
	Outcome     GuardrailOutcomeType
	ReasonCode  string
	At          time.Time
}

type ReadinessScope string

const (
	ReadinessScopePIDPolicy            ReadinessScope = "pid_policy"
	ReadinessScopeGatewayBoundary      ReadinessScope = "gateway_boundary"
	ReadinessScopeLocationTransparency ReadinessScope = "location_transparency"
)

type ReadinessResult string

const (
	ReadinessPass ReadinessResult = "pass"
	ReadinessFail ReadinessResult = "fail"
)

type ReadinessValidationRecord struct {
	Scope       ReadinessScope
	Result      ReadinessResult
	CheckedAt   time.Time
	EvidenceRef string
}

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
	Ask           *AskRequestContext
	TypeName      string
	SchemaVersion string
	AcceptedAt    time.Time
	Attempt       int
}

func (m Message) IsAsk() bool {
	return m.Ask != nil
}

func (m Message) AskRequestID() string {
	if m.Ask == nil {
		return ""
	}
	return m.Ask.RequestID
}

func (m Message) AskReplyTo() (PID, bool) {
	if m.Ask == nil {
		return PID{}, false
	}
	return m.Ask.ReplyTo, true
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

type AskOutcomeType string

const (
	AskOutcomeSuccess            AskOutcomeType = "ask_success"
	AskOutcomeTimeout            AskOutcomeType = "ask_timeout"
	AskOutcomeCanceled           AskOutcomeType = "ask_canceled"
	AskOutcomeReplyTargetInvalid AskOutcomeType = "ask_reply_target_invalid"
	AskOutcomeLateReplyDropped   AskOutcomeType = "ask_late_reply_dropped"
)

type AskRequestContext struct {
	RequestID string
	ReplyTo   PID
}

type AskReplyEnvelope struct {
	RequestID string
	Payload   any
	RepliedAt time.Time
}

type AskResult struct {
	RequestID string
	Payload   any
}

type AskOutcome struct {
	RequestID   string
	ActorID     string
	ReplyTo     PID
	Outcome     AskOutcomeType
	ReasonCode  string
	CompletedAt time.Time
}

type RouterStrategy string

const (
	RouterStrategyRoundRobin    RouterStrategy = "round_robin"
	RouterStrategyRandom        RouterStrategy = "random"
	RouterStrategyConsistentKey RouterStrategy = "consistent_hash"
)

type HashKeyMessage interface {
	HashKey() string
}

type RoutingOutcomeType string

const (
	RouteSuccess                 RoutingOutcomeType = "route_success"
	RouteFailedNoWorkers         RoutingOutcomeType = "route_failed_no_workers"
	RouteFailedInvalidKey        RoutingOutcomeType = "route_failed_invalid_key"
	RouteFailedWorkerUnavailable RoutingOutcomeType = "route_failed_worker_unavailable"
)

type RouterDefinition struct {
	RouterID string
	Strategy RouterStrategy
	Workers  []string
}

type RoutingOutcome struct {
	RouterID       string
	MessageID      uint64
	Strategy       RouterStrategy
	SelectedWorker string
	Outcome        RoutingOutcomeType
	ReasonCode     string
	At             time.Time
}

type BatchReceive interface {
	BatchReceive(ctx context.Context, state any, payloads []any) (nextState any, err error)
}

type BatchConfig struct {
	Enabled  bool
	MaxSize  int
	Receiver BatchReceive
}

type BatchResult string

const (
	BatchResultSuccess           BatchResult = "batch_success"
	BatchResultFailedHandler     BatchResult = "batch_failed_handler"
	BatchResultFailedSupervision BatchResult = "batch_failed_supervision"
)

type BatchOutcome struct {
	ActorID     string
	BatchSize   int
	Result      BatchResult
	ReasonCode  string
	CompletedAt time.Time
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
