package actor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Sentinel errors returned by the actor runtime.
var (
	// ErrDuplicateActorID is returned when creating an actor with an ID that already exists.
	ErrDuplicateActorID = errors.New("duplicate_actor_id")
	// ErrActorNotFound is returned when the target actor does not exist.
	ErrActorNotFound = errors.New("actor_not_found")
	// ErrActorStopped is returned when sending to a stopped actor.
	ErrActorStopped = errors.New("actor_stopped")
	// ErrActorStillRunning is returned by RemoveActor when the actor has not been stopped yet.
	ErrActorStillRunning = errors.New("actor_still_running")
	// ErrStateMutationForbidden is returned when attempting to set state via ActorRef.
	ErrStateMutationForbidden = errors.New("state_mutation_forbidden")
	// ErrRegistryNameInvalid is returned when a registry name fails validation.
	ErrRegistryNameInvalid = errors.New("registry_name_invalid")
	// ErrRegistryDuplicateName is returned when registering a name that is already taken.
	ErrRegistryDuplicateName = errors.New("registry_duplicate_name")
	// ErrRegistryNameNotFound is returned when looking up an unregistered name.
	ErrRegistryNameNotFound = errors.New("registry_name_not_found")
	// ErrLifecycleStartFailed is returned when an actor's start hook fails.
	ErrLifecycleStartFailed = errors.New("lifecycle_start_failed")
	// ErrLifecycleStopFailed is returned when an actor's stop hook fails.
	ErrLifecycleStopFailed = errors.New("lifecycle_stop_failed")
	// ErrNonPIDCrossActor is returned when a cross-actor send is rejected by PID-only policy.
	ErrNonPIDCrossActor = errors.New("non_pid_cross_actor_rejected")
	// ErrGatewayRouteFailed is returned when gateway-mediated routing fails.
	ErrGatewayRouteFailed = errors.New("gateway_route_failed")
	// ErrAskTimeout is returned when an ask request exceeds its deadline.
	ErrAskTimeout = errors.New("ask_timeout")
	// ErrAskCanceled is returned when an ask request's context is canceled.
	ErrAskCanceled = errors.New("ask_canceled")
	// ErrAskInvalidTimeout is returned when an ask timeout is zero or negative.
	ErrAskInvalidTimeout = errors.New("ask_invalid_timeout")
	// ErrAskReplyTargetInvalid is returned when the ask reply target cannot be resolved.
	ErrAskReplyTargetInvalid = errors.New("ask_reply_target_invalid")
	// ErrBatchConfigInvalid is returned when batch configuration is invalid.
	ErrBatchConfigInvalid = errors.New("batch_config_invalid")
	// ErrPubSubInvalidCommand is returned when a pubsub command is malformed.
	ErrPubSubInvalidCommand = errors.New("pubsub_invalid_command")
	// ErrPubSubInvalidPattern is returned when a pubsub subscription pattern is invalid.
	ErrPubSubInvalidPattern = errors.New("pubsub_invalid_pattern")
	// ErrPubSubInvalidTopic is returned when a pubsub topic is invalid.
	ErrPubSubInvalidTopic = errors.New("pubsub_invalid_topic")
)

// ActorStatus represents the lifecycle state of an actor.
type ActorStatus string

const (
	// ActorStarting indicates the actor is executing its start hook.
	ActorStarting ActorStatus = "starting"
	// ActorRunning indicates the actor is processing messages.
	ActorRunning ActorStatus = "running"
	// ActorRestarting indicates the actor is being restarted by the supervisor.
	ActorRestarting ActorStatus = "restarting"
	// ActorStopping indicates the actor is executing its stop hook.
	ActorStopping ActorStatus = "stopping"
	// ActorStopped indicates the actor has terminated.
	ActorStopped ActorStatus = "stopped"
)

// LifecycleHookPhase identifies whether a lifecycle hook runs at start or stop.
type LifecycleHookPhase string

const (
	// LifecyclePhaseStart is the phase for actor start hooks.
	LifecyclePhaseStart LifecycleHookPhase = "start"
	// LifecyclePhaseStop is the phase for actor stop hooks.
	LifecyclePhaseStop LifecycleHookPhase = "stop"
)

// LifecycleHookResult describes the outcome of a lifecycle hook execution.
type LifecycleHookResult string

const (
	// LifecycleStartSuccess indicates the start hook completed without error.
	LifecycleStartSuccess LifecycleHookResult = "start_success"
	// LifecycleStartFailed indicates the start hook returned an error or panicked.
	LifecycleStartFailed LifecycleHookResult = "start_failed"
	// LifecycleStopSuccess indicates the stop hook completed without error.
	LifecycleStopSuccess LifecycleHookResult = "stop_success"
	// LifecycleStopFailed indicates the stop hook returned an error, panicked, or timed out.
	LifecycleStopFailed LifecycleHookResult = "stop_failed"
)

// LifecycleHook is a callback invoked during actor start or stop.
type LifecycleHook func(ctx context.Context, actorID string) error

// LifecycleHookOutcome records the result of a lifecycle hook invocation.
type LifecycleHookOutcome struct {
	ActorID     string
	Phase       LifecycleHookPhase
	Result      LifecycleHookResult
	CompletedAt time.Time
	ErrorCode   string
}

// PIDInteractionPolicyMode controls whether cross-actor sends require PID addressing.
type PIDInteractionPolicyMode string

const (
	// PIDInteractionPolicyDisabled allows cross-actor sends by actor ID.
	PIDInteractionPolicyDisabled PIDInteractionPolicyMode = "disabled"
	// PIDInteractionPolicyPIDOnly rejects cross-actor sends that use actor ID instead of PID.
	PIDInteractionPolicyPIDOnly PIDInteractionPolicyMode = "pid_only"
)

// GatewayRouteMode controls how PID-addressed messages are routed.
type GatewayRouteMode string

const (
	// GatewayRouteLocalDirect routes messages directly to local actors.
	GatewayRouteLocalDirect GatewayRouteMode = "local_direct"
	// GatewayRouteGatewayMediated routes messages through a gateway boundary check.
	GatewayRouteGatewayMediated GatewayRouteMode = "gateway_mediated"
)

// GuardrailOutcomeType categorizes the result of a guardrail policy evaluation.
type GuardrailOutcomeType string

const (
	// GuardrailPolicyAccept indicates the guardrail accepted the interaction.
	GuardrailPolicyAccept GuardrailOutcomeType = "policy_accept"
	// GuardrailPolicyRejectNonPID indicates the guardrail rejected a non-PID cross-actor send.
	GuardrailPolicyRejectNonPID GuardrailOutcomeType = "policy_reject_non_pid"
	// GuardrailGatewayRouteSuccess indicates the gateway routing succeeded.
	GuardrailGatewayRouteSuccess GuardrailOutcomeType = "gateway_route_success"
	// GuardrailGatewayRouteFailure indicates the gateway routing failed.
	GuardrailGatewayRouteFailure GuardrailOutcomeType = "gateway_route_failure"
)

// GuardrailOutcome records the result of a guardrail policy evaluation for an actor interaction.
type GuardrailOutcome struct {
	ActorID     string
	PolicyMode  PIDInteractionPolicyMode
	GatewayMode GatewayRouteMode
	Outcome     GuardrailOutcomeType
	ReasonCode  string
	At          time.Time
}

// ReadinessScope identifies the category of a distributed readiness check.
type ReadinessScope string

const (
	// ReadinessScopePIDPolicy checks that PID interaction policy is enforced.
	ReadinessScopePIDPolicy ReadinessScope = "pid_policy"
	// ReadinessScopeGatewayBoundary checks that the gateway route mode is valid.
	ReadinessScopeGatewayBoundary ReadinessScope = "gateway_boundary"
	// ReadinessScopeLocationTransparency checks location-transparent addressing readiness.
	ReadinessScopeLocationTransparency ReadinessScope = "location_transparency"
)

// ReadinessResult indicates whether a readiness check passed or failed.
type ReadinessResult string

const (
	// ReadinessPass indicates the readiness check passed.
	ReadinessPass ReadinessResult = "pass"
	// ReadinessFail indicates the readiness check failed.
	ReadinessFail ReadinessResult = "fail"
)

// ReadinessValidationRecord captures the result of a single readiness validation check.
type ReadinessValidationRecord struct {
	Scope       ReadinessScope
	Result      ReadinessResult
	CheckedAt   time.Time
	EvidenceRef string
}

// SubmitResult describes the outcome of submitting a message to an actor's mailbox.
type SubmitResult string

const (
	// SubmitAccepted indicates the message was enqueued successfully.
	SubmitAccepted SubmitResult = "accepted"
	// SubmitRejectedFull indicates the mailbox is at capacity.
	SubmitRejectedFull SubmitResult = "rejected_full"
	// SubmitRejectedStop indicates the target actor is stopped.
	SubmitRejectedStop SubmitResult = "rejected_stopped"
	// SubmitRejectedFound indicates the target actor was not found.
	SubmitRejectedFound SubmitResult = "rejected_not_found"
	// SubmitRejectedUnsupportedType indicates the message type is not accepted by the actor's routing rules.
	SubmitRejectedUnsupportedType SubmitResult = "rejected_unsupported_type"
	// SubmitRejectedNilPayload indicates the message payload was nil.
	SubmitRejectedNilPayload SubmitResult = "rejected_nil_payload"
	// SubmitRejectedVersionMismatch indicates the message schema version does not match.
	SubmitRejectedVersionMismatch SubmitResult = "rejected_version_mismatch"
)

// StopResult describes the outcome of stopping an actor.
type StopResult string

const (
	// StopStopped indicates the actor was successfully stopped.
	StopStopped StopResult = "stopped"
	// StopAlready indicates the actor was already stopped.
	StopAlready StopResult = "already_stopped"
	// StopNotFound indicates the actor does not exist.
	StopNotFound StopResult = "not_found"
)

// Message is the envelope delivered to an actor's handler.
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

// IsAsk returns true if this message is part of a request-response (ask) interaction.
func (m Message) IsAsk() bool {
	return m.Ask != nil
}

// AskRequestID returns the ask request ID, or empty string if this is not an ask message.
func (m Message) AskRequestID() string {
	if m.Ask == nil {
		return ""
	}
	return m.Ask.RequestID
}

// AskReplyTo returns the PID to send the reply to, or false if this is not an ask message.
func (m Message) AskReplyTo() (PID, bool) {
	if m.Ask == nil {
		return PID{}, false
	}
	return m.Ask.ReplyTo, true
}

// ProcessingResult describes the outcome of processing a message by an actor.
type ProcessingResult string

const (
	// ResultSuccess indicates the handler completed successfully.
	ResultSuccess ProcessingResult = "success"
	// ResultDelivered indicates the message was delivered and processed.
	ResultDelivered ProcessingResult = "delivered"
	// ResultFailed indicates the handler returned an error or panicked.
	ResultFailed ProcessingResult = "failed"
	// ResultRejectedFull indicates the mailbox was full.
	ResultRejectedFull ProcessingResult = "rejected_full"
	// ResultRejectedStop indicates the actor was stopped.
	ResultRejectedStop ProcessingResult = "rejected_stopped"
	// ResultRejectedFound indicates the actor was not found.
	ResultRejectedFound ProcessingResult = "rejected_not_found"
	// ResultRejectedUnsupportedType indicates the message type was not accepted.
	ResultRejectedUnsupportedType ProcessingResult = "rejected_unsupported_type"
	// ResultRejectedNilPayload indicates the payload was nil.
	ResultRejectedNilPayload ProcessingResult = "rejected_nil_payload"
	// ResultRejectedVersionMismatch indicates a schema version mismatch.
	ResultRejectedVersionMismatch ProcessingResult = "rejected_version_mismatch"
)

// ProcessingOutcome records the full result of processing a single message, including supervision decisions.
type ProcessingOutcome struct {
	MessageID            uint64
	ActorID              string
	Result               ProcessingResult
	SupervisionDecision  SupervisionDecision
	SupervisionIteration int
	CompletedAt          time.Time
	ErrorCode            string
}

// AskOutcomeType categorizes the result of a request-response (ask) interaction.
type AskOutcomeType string

const (
	// AskOutcomeSuccess indicates the ask received a reply.
	AskOutcomeSuccess AskOutcomeType = "ask_success"
	// AskOutcomeTimeout indicates the ask timed out waiting for a reply.
	AskOutcomeTimeout AskOutcomeType = "ask_timeout"
	// AskOutcomeCanceled indicates the ask was canceled via context.
	AskOutcomeCanceled AskOutcomeType = "ask_canceled"
	// AskOutcomeReplyTargetInvalid indicates the reply target could not be resolved.
	AskOutcomeReplyTargetInvalid AskOutcomeType = "ask_reply_target_invalid"
	// AskOutcomeLateReplyDropped indicates a reply arrived after the ask was already completed.
	AskOutcomeLateReplyDropped AskOutcomeType = "ask_late_reply_dropped"
)

// AskRequestContext carries the metadata for an in-flight ask request.
type AskRequestContext struct {
	RequestID string
	ReplyTo   PID
}

// AskReplyEnvelope wraps a reply payload sent back to the ask caller.
type AskReplyEnvelope struct {
	RequestID string
	Payload   any
	RepliedAt time.Time
}

// AskResult holds the reply payload returned from a successful ask.
type AskResult struct {
	RequestID string
	Payload   any
}

// AskOutcome records the result of a completed ask interaction.
type AskOutcome struct {
	RequestID   string
	ActorID     string
	ReplyTo     PID
	Outcome     AskOutcomeType
	ReasonCode  string
	CompletedAt time.Time
}

// RouterStrategy defines the algorithm a router uses to select a worker.
type RouterStrategy string

const (
	// RouterStrategyRoundRobin distributes messages to workers in round-robin order.
	RouterStrategyRoundRobin RouterStrategy = "round_robin"
	// RouterStrategyRandom distributes messages to a random worker.
	RouterStrategyRandom RouterStrategy = "random"
	// RouterStrategyConsistentKey routes messages by consistent hashing of the payload's HashKey.
	RouterStrategyConsistentKey RouterStrategy = "consistent_hash"
)

// HashKeyMessage is implemented by payloads that provide a key for consistent-hash routing.
type HashKeyMessage interface {
	HashKey() string
}

// RoutingOutcomeType categorizes the result of a router dispatch.
type RoutingOutcomeType string

const (
	// RouteSuccess indicates the message was routed to a worker.
	RouteSuccess RoutingOutcomeType = "route_success"
	// RouteFailedNoWorkers indicates the router has no configured workers.
	RouteFailedNoWorkers RoutingOutcomeType = "route_failed_no_workers"
	// RouteFailedInvalidKey indicates the payload is missing or has an invalid hash key.
	RouteFailedInvalidKey RoutingOutcomeType = "route_failed_invalid_key"
	// RouteFailedWorkerUnavailable indicates the selected worker could not accept the message.
	RouteFailedWorkerUnavailable RoutingOutcomeType = "route_failed_worker_unavailable"
)

// RouterDefinition describes a router's configuration.
type RouterDefinition struct {
	RouterID string
	Strategy RouterStrategy
	Workers  []string
}

// RoutingOutcome records the result of a single router dispatch.
type RoutingOutcome struct {
	RouterID       string
	MessageID      uint64
	Strategy       RouterStrategy
	SelectedWorker string
	Outcome        RoutingOutcomeType
	ReasonCode     string
	At             time.Time
}

// BatchReceive is implemented by actors that process messages in batches.
type BatchReceive interface {
	BatchReceive(ctx context.Context, state any, payloads []any) (nextState any, err error)
}

// BatchConfig holds the batching configuration for an actor.
type BatchConfig struct {
	Enabled  bool
	MaxSize  int
	Receiver BatchReceive
}

// BatchResult describes the outcome of processing a batch of messages.
type BatchResult string

const (
	// BatchResultSuccess indicates the batch was processed successfully.
	BatchResultSuccess BatchResult = "batch_success"
	// BatchResultFailedHandler indicates the batch handler returned an error.
	BatchResultFailedHandler BatchResult = "batch_failed_handler"
	// BatchResultFailedSupervision indicates the batch failed and supervision took action.
	BatchResultFailedSupervision BatchResult = "batch_failed_supervision"
)

// BatchOutcome records the result of processing a single batch.
type BatchOutcome struct {
	ActorID     string
	BatchSize   int
	Result      BatchResult
	ReasonCode  string
	CompletedAt time.Time
}

// Handler is the function invoked for each message delivered to an actor.
// It receives the current state and returns the next state.
type Handler func(ctx context.Context, state any, msg Message) (nextState any, err error)

// SupervisionDecision is the action a supervisor takes when an actor fails.
type SupervisionDecision string

const (
	// DecisionRestart restarts the actor with its initial state.
	DecisionRestart SupervisionDecision = "restart"
	// DecisionStop permanently stops the actor.
	DecisionStop SupervisionDecision = "stop"
	// DecisionEscalate stops the actor and signals the failure to a parent.
	DecisionEscalate SupervisionDecision = "escalate"
)

// RegistryOperationResult describes the outcome of a named registry operation.
type RegistryOperationResult string

const (
	// RegistryRegisterSuccess indicates the name was registered.
	RegistryRegisterSuccess RegistryOperationResult = "register_success"
	// RegistryRegisterRejectedDup indicates the name is already registered.
	RegistryRegisterRejectedDup RegistryOperationResult = "register_rejected_duplicate"
	// RegistryLookupHit indicates the name was found.
	RegistryLookupHit RegistryOperationResult = "lookup_hit"
	// RegistryLookupNotFound indicates the name was not found.
	RegistryLookupNotFound RegistryOperationResult = "lookup_not_found"
	// RegistryUnregisterSuccess indicates the name was unregistered.
	RegistryUnregisterSuccess RegistryOperationResult = "unregister_success"
	// RegistryUnregisterLifecycleTerm indicates the name was unregistered due to actor termination.
	RegistryUnregisterLifecycleTerm RegistryOperationResult = "unregister_lifecycle_terminal"
)

// RegistryRegisterAck is the acknowledgment returned after a name registration attempt.
type RegistryRegisterAck struct {
	Result RegistryOperationResult
	Name   string
	PID    PID
}

// RegistryLookupAck is the acknowledgment returned after a name lookup.
type RegistryLookupAck struct {
	Result RegistryOperationResult
	Name   string
	PID    PID
}

// BrokerSubscribeCommand requests a PID subscription to a topic pattern.
type BrokerSubscribeCommand struct {
	Subscriber PID
	Pattern    string
}

// BrokerUnsubscribeCommand requests removal of a PID subscription from a topic pattern.
type BrokerUnsubscribeCommand struct {
	Subscriber PID
	Pattern    string
}

// BrokerPublishCommand publishes a message to a topic.
type BrokerPublishCommand struct {
	Topic            string
	Payload          any
	PublisherActorID string
}

// BrokerPublishedMessage is delivered to subscribers when a topic receives a publication.
type BrokerPublishedMessage struct {
	Topic            string
	Payload          any
	PublisherActorID string
	PublishedAt      time.Time
}

// BrokerOperation identifies the type of pubsub broker command.
type BrokerOperation string

const (
	// BrokerOperationSubscribe is a subscribe operation.
	BrokerOperationSubscribe BrokerOperation = "subscribe"
	// BrokerOperationUnsubscribe is an unsubscribe operation.
	BrokerOperationUnsubscribe BrokerOperation = "unsubscribe"
	// BrokerOperationPublish is a publish operation.
	BrokerOperationPublish BrokerOperation = "publish"
)

// BrokerOutcomeType categorizes the result of a pubsub broker operation.
type BrokerOutcomeType string

const (
	// BrokerOutcomeSubscribeSuccess indicates a subscription was created.
	BrokerOutcomeSubscribeSuccess BrokerOutcomeType = "subscribe_success"
	// BrokerOutcomeUnsubscribeSuccess indicates a subscription was removed.
	BrokerOutcomeUnsubscribeSuccess BrokerOutcomeType = "unsubscribe_success"
	// BrokerOutcomePublishSuccess indicates the publication was delivered to all subscribers.
	BrokerOutcomePublishSuccess BrokerOutcomeType = "publish_success"
	// BrokerOutcomePublishPartialDelivery indicates some subscribers could not be reached.
	BrokerOutcomePublishPartialDelivery BrokerOutcomeType = "publish_partial_delivery"
	// BrokerOutcomeInvalidCommand indicates the broker command was malformed.
	BrokerOutcomeInvalidCommand BrokerOutcomeType = "invalid_command"
	// BrokerOutcomeInvalidPattern indicates the subscription pattern was invalid.
	BrokerOutcomeInvalidPattern BrokerOutcomeType = "invalid_pattern"
	// BrokerOutcomeInvalidTopic indicates the publish topic was invalid.
	BrokerOutcomeInvalidTopic BrokerOutcomeType = "invalid_topic"
	// BrokerOutcomeTargetUnreachable indicates a subscriber could not be reached.
	BrokerOutcomeTargetUnreachable BrokerOutcomeType = "target_unreachable"
	// BrokerOutcomeNotFoundNoop indicates the subscription was not found (no-op).
	BrokerOutcomeNotFoundNoop BrokerOutcomeType = "not_found_noop"
)

// BrokerCommandAck is the acknowledgment returned after a broker command.
type BrokerCommandAck struct {
	Operation    BrokerOperation
	Result       BrokerOutcomeType
	TopicPattern string
	MatchedCount int
	ReasonCode   string
}

// BrokerOutcome records the result of a pubsub broker operation.
type BrokerOutcome struct {
	BrokerID     string
	Operation    BrokerOperation
	TopicPattern string
	Subscriber   PID
	MatchedCount int
	Result       BrokerOutcomeType
	ReasonCode   string
	At           time.Time
}

var registryNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func validateRegistryName(name string) error {
	if !registryNamePattern.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrRegistryNameInvalid, name)
	}
	return nil
}
