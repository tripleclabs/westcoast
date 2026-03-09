# Data Model: Supervisor Trees & Fault Tolerance

## Entity: SupervisorPolicy
- Fields:
  - `max_restarts` (int)
  - `decision_on_failure` (enum: `restart`, `stop`, `escalate`)
  - `window_seconds` (optional int for restart-rate control)
- Relationships:
  - Applied to one or many actors.
- Validation Rules:
  - `max_restarts` MUST be >= 0.
  - Decisions MUST be explicit and deterministic.

## Entity: FailureRecord
- Fields:
  - `actor_id` (string)
  - `message_id` (uint64)
  - `failure_class` (enum: `panic`, `handler_error`)
  - `decision` (enum: `restart`, `stop`, `escalate`)
  - `restart_count` (int)
  - `occurred_at` (timestamp)
- Relationships:
  - Produced by one actor failure episode.
  - Links to one terminal ProcessingOutcome.
- Validation Rules:
  - Exactly one decision per failure episode.
  - Event payload MUST include actor and message identity.

## Entity: RestartLifecycle
- Fields:
  - `actor_id` (string)
  - `previous_status` (enum)
  - `next_status` (enum)
  - `transition_at` (timestamp)
  - `trigger_message_id` (uint64)
- Relationships:
  - Multiple lifecycle transitions can occur per actor over time.
- Validation Rules:
  - Restart flow transitions MUST be ordered and observable.

## Entity: MailboxSnapshot
- Fields:
  - `actor_id` (string)
  - `pending_messages` (ordered list of Message IDs/payload references)
  - `captured_at` (timestamp)
- Relationships:
  - Tied to actor restart episodes.
- Validation Rules:
  - Unprocessed queued messages MUST remain in FIFO order after restart.
  - Crash-triggering message MUST not be reclassified as preserved if already failed.

## State Transitions

### Actor Failure & Recovery Lifecycle
1. `running -> failed` when panic/error occurs during message handling.
2. `failed -> restarting` when supervisor returns `restart` within limits.
3. `restarting -> running` after initial state reset and actor loop resumes.
4. `failed -> stopped` when supervisor returns `stop` or restart limits are exceeded with stop policy.
5. `failed -> escalated` when supervisor returns `escalate`.

### Mailbox Handling During Restart
1. `queued -> preserved` for unprocessed mailbox messages at crash point.
2. `processing -> failed` for crash-triggering message.
3. `preserved -> processing` after actor resumes.
4. `processing -> completed/rejected` per normal processing outcomes.
