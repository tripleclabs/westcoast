# Contract: Supervisor Trees & Fault Tolerance

## Purpose
Define caller-visible and operator-visible guarantees for panic interception, supervision decisions,
state resets, and mailbox preservation in actor restart flows.

## Failure Isolation Contract
- Panic or fatal handler failure in one actor MUST NOT terminate runtime process.
- Failures are scoped to the crashing actor and its supervision decision path.
- Healthy actors continue processing messages during unrelated actor failures.

## Supervision Decision Contract

### Supported Decisions
- `restart`
- `stop`
- `escalate`

### Decision Guarantees
- Exactly one supervision decision MUST be applied per failure episode.
- Decision MUST be deterministic given policy inputs and restart counters.
- Post-limit behavior MUST be explicit and testable.

## Restart Contract
- For `restart`, actor state MUST reset to configured initial snapshot.
- Restarted actor MUST return to running status if restart succeeds.
- Crash-triggering message outcome MUST be recorded as failed and not silently dropped.

## Mailbox Preservation Contract
- Unprocessed queued messages at crash point MUST be preserved across restart.
- Preserved messages MUST maintain FIFO ordering relative to queue state at crash point.
- New messages arriving during restart MUST be integrated without violating queue ordering guarantees.

## Location Transparency Contract
- Caller-facing actor addressing remains logical (`actor_id`, PID abstraction).
- Restart does not require callers to switch to process-local references.
- Future distributed routing can replace transport without changing caller-facing failure semantics.

## Event Contract
Required event types:
- `actor_failed`
- `actor_restarted`
- `actor_stopped` or `actor_escalated`
- `message_processed` / `message_rejected` (existing outcome continuity)

Required event fields:
- `event_id`
- `actor_id`
- `message_id`
- `result`
- `error_code` (if present)
- `timestamp`
- `supervision_decision`
- `restart_count`

## Invariants
- Let-it-crash behavior is explicit and actor-scoped.
- Clean state restart prevents corrupted state carry-forward.
- Mailbox preservation semantics are deterministic and testable.
