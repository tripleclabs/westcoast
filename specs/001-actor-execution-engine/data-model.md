# Data Model: Actor Execution Engine

## Entity: Actor
- Fields:
  - `actor_id` (string, immutable, unique)
  - `status` (enum: `starting`, `running`, `restarting`, `stopped`)
  - `mailbox_id` (string, immutable)
  - `restart_count` (integer >= 0)
  - `created_at` (timestamp)
  - `last_transition_at` (timestamp)
- Relationships:
  - Owns exactly one Mailbox.
  - Processes zero-to-many Messages over lifecycle.
  - Emits zero-to-many ProcessingOutcome records.
- Validation Rules:
  - `actor_id` MUST be unique runtime-wide.
  - `status` transitions MUST follow allowed state machine.

## Entity: ActorState
- Fields:
  - `actor_id` (string, unique foreign key to Actor)
  - `revision` (integer >= 0)
  - `payload` (opaque actor-owned value)
- Relationships:
  - One-to-one with Actor.
- Validation Rules:
  - Mutations allowed only within actor message-processing context.
  - External write attempts MUST be rejected.

## Entity: Mailbox
- Fields:
  - `mailbox_id` (string, unique)
  - `actor_id` (string, unique foreign key)
  - `capacity` (integer > 0)
  - `depth` (integer >= 0, <= capacity)
  - `policy` (enum: `reject_on_full`)
- Relationships:
  - Belongs to one Actor.
  - Contains queued Messages.
- Validation Rules:
  - Enqueue MUST return immediately with acceptance/rejection outcome.
  - Dequeue order MUST preserve acceptance order.

## Entity: Message
- Fields:
  - `message_id` (string, unique)
  - `actor_id` (string, foreign key)
  - `accepted_at` (timestamp)
  - `payload` (opaque user value)
  - `attempt` (integer >= 1)
- Relationships:
  - Targets one Actor.
  - Resides in one Mailbox until processed or dropped.
- Validation Rules:
  - Ordering for one actor MUST be FIFO by `accepted_at`.
  - Message for missing/stopped actor MUST return explicit rejection outcome.

## Entity: ProcessingOutcome
- Fields:
  - `message_id` (string, unique foreign key)
  - `actor_id` (string, foreign key)
  - `result` (enum: `success`, `failed`, `rejected_full`, `rejected_stopped`)
  - `completed_at` (timestamp)
  - `error_code` (string, optional)
- Relationships:
  - Produced by Actor processing of one Message.
- Validation Rules:
  - Every accepted message MUST emit exactly one terminal outcome.

## State Transitions

### Actor Lifecycle
1. `starting -> running` when mailbox loop begins.
2. `running -> restarting` when supervised failure is detected.
3. `restarting -> running` when restart succeeds.
4. `running -> stopped` when normal stop requested.
5. `restarting -> stopped` when restart policy halts actor.

### Message Lifecycle
1. `submitted -> accepted` when mailbox has capacity.
2. `submitted -> rejected_full` when capacity reached.
3. `submitted -> rejected_stopped` when actor unavailable.
4. `accepted -> processing` when actor dequeues.
5. `processing -> success|failed` at terminal completion.
