package actor

import (
	"errors"
	"time"
)

// Sentinel errors for cron operations.
var (
	// ErrCronInvalidSchedule is returned when a cron expression cannot be parsed.
	ErrCronInvalidSchedule = errors.New("cron_invalid_schedule")
	// ErrCronInvalidCommand is returned when a cron command is malformed.
	ErrCronInvalidCommand = errors.New("cron_invalid_command")
	// ErrCronRefNotFound is returned when a cron subscription ref does not exist.
	ErrCronRefNotFound = errors.New("cron_ref_not_found")
)

// CronRef is a handle to a registered cron subscription.
type CronRef struct {
	ID       string
	Schedule string
	ActorID  string
}

// CronSubscribeCommand requests a cron-scheduled callback. Must be sent via Ask.
type CronSubscribeCommand struct {
	Subscriber PID
	Schedule   string // cron expression (5-field standard, with optional seconds, or @every)
	Payload    any    // delivered on each tick
	Tag        string // optional human-readable label
}

// CronUnsubscribeCommand cancels a cron subscription by ref ID.
type CronUnsubscribeCommand struct {
	Subscriber PID
	RefID      string
}

// CronUnsubscribeAllCommand cancels all cron subscriptions for a subscriber.
type CronUnsubscribeAllCommand struct {
	Subscriber PID
}

// CronListCommand lists active cron subscriptions for a subscriber.
type CronListCommand struct {
	Subscriber PID
}

// CronTickMessage is delivered to a subscriber when their cron schedule fires.
type CronTickMessage struct {
	Ref     CronRef
	Payload any
	FiredAt time.Time
	Tag     string
}

// CronOperation identifies the type of cron command.
type CronOperation string

const (
	CronOperationSubscribe      CronOperation = "subscribe"
	CronOperationUnsubscribe    CronOperation = "unsubscribe"
	CronOperationUnsubscribeAll CronOperation = "unsubscribe_all"
	CronOperationList           CronOperation = "list"
)

// CronOutcomeType categorizes the result of a cron operation.
type CronOutcomeType string

const (
	CronOutcomeSubscribeSuccess   CronOutcomeType = "subscribe_success"
	CronOutcomeUnsubscribeSuccess CronOutcomeType = "unsubscribe_success"
	CronOutcomeListSuccess        CronOutcomeType = "list_success"
	CronOutcomeInvalidSchedule    CronOutcomeType = "invalid_schedule"
	CronOutcomeInvalidCommand     CronOutcomeType = "invalid_command"
	CronOutcomeNotFound           CronOutcomeType = "not_found"
)

// CronCommandAck is the reply to a cron command (via Ask).
type CronCommandAck struct {
	Operation  CronOperation
	Result     CronOutcomeType
	Ref        CronRef   // populated on subscribe success
	Refs       []CronRef // populated on list success
	ReasonCode string
}

// CronHandoffState is the serializable state transferred during singleton handoff.
type CronHandoffState struct {
	Entries []CronHandoffEntry
}

// CronHandoffEntry represents a single cron subscription in handoff state.
type CronHandoffEntry struct {
	RefID    string
	ActorID  string
	Schedule string
	Tag      string
	Payload  any
}
