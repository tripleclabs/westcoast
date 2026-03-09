# Data Model: Request-Response Ask Semantics

## Entity: AskRequestContext
- Fields:
  - `request_id` (string)
  - `reply_to` (PID)
  - `initiator_actor_id` (string, optional)
  - `created_at` (timestamp)
- Relationships:
  - Attached to each Ask-originated message.
  - References one `ReplyEndpoint` through `reply_to`.
- Validation Rules:
  - `request_id` MUST be unique among active wait slots.
  - `reply_to` MUST be valid PID for the request lifecycle.

## Entity: AskWaitSlot
- Fields:
  - `request_id` (string)
  - `status` (enum: `pending`, `completed`, `timed_out`, `canceled`)
  - `deadline_at` (timestamp)
  - `completed_at` (timestamp, optional)
  - `result_type` (enum: `reply`, `timeout`, `canceled`, `delivery_failed`)
- Relationships:
  - Created by Ask caller path and completed by first valid `AskReplyEnvelope` or timeout/cancel path.
- Validation Rules:
  - Transition from `pending` MUST occur exactly once.
  - Completed/timed_out/canceled slots MUST be removed or retained under bounded policy.

## Entity: ReplyEndpoint
- Fields:
  - `reply_to` (PID)
  - `request_id` (string)
  - `expiry_at` (timestamp)
  - `active` (boolean)
- Relationships:
  - One-to-one with active `AskWaitSlot`.
- Validation Rules:
  - Endpoint MUST accept at most one correlating reply for active slot.
  - Endpoint MUST reject/ignore replies after slot completion or expiry.

## Entity: AskReplyEnvelope
- Fields:
  - `request_id` (string)
  - `payload` (opaque message payload)
  - `replied_at` (timestamp)
- Relationships:
  - Produced by responder using `ReplyTo` PID.
  - Consumed by `AskWaitSlot` matching `request_id`.
- Validation Rules:
  - Envelope `request_id` MUST match active pending slot.
  - Duplicate envelopes for completed slot MUST not produce second completion.

## Entity: AskOutcomeRecord
- Fields:
  - `event_id` (uint64)
  - `request_id` (string)
  - `actor_id` (string)
  - `outcome` (enum: `ask_success`, `ask_timeout`, `ask_canceled`, `ask_reply_target_invalid`, `ask_late_reply_dropped`)
  - `reason_code` (string, optional)
  - `timestamp` (timestamp)
- Relationships:
  - Emitted from Ask lifecycle transitions and reply-routing checks.
- Validation Rules:
  - Every Ask request MUST emit one terminal caller outcome (`ask_success`, `ask_timeout`, `ask_canceled`, or `ask_reply_target_invalid`).
  - Late reply drops MUST emit `ask_late_reply_dropped` without altering terminal outcome.

## State Transitions

### Ask Wait Lifecycle
1. `pending -> completed` when first valid reply is correlated.
2. `pending -> timed_out` when deadline expires before valid reply.
3. `pending -> canceled` when caller context cancellation occurs first.
4. `pending -> delivery_failed` when reply path cannot be established.

### Reply Endpoint Lifecycle
1. `active -> inactive` on first successful correlation.
2. `active -> inactive` on timeout or cancellation.
3. `inactive` endpoints reject/ignore further replies and record late-drop outcome.

### Outcome Emission Lifecycle
1. Ask call starts with non-terminal telemetry only.
2. Terminal transition emits exactly one caller-visible terminal outcome.
3. Non-terminal post-completion events (e.g., late replies) emit auxiliary outcomes without reopening slot.
