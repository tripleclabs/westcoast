# Data Model: Smart Mailboxes and Message Batching

## Entity: BatchMailboxConfig
- Fields:
  - `actor_id` (string)
  - `enabled` (boolean)
  - `max_batch_size` (integer)
  - `updated_at` (timestamp)
- Relationships:
  - Applied by runtime to one actor mailbox execution path.
- Validation Rules:
  - `max_batch_size` MUST be greater than zero when batching is enabled.
  - Disabled batching MUST preserve existing single-message behavior.

## Entity: BatchEnvelope
- Fields:
  - `actor_id` (string)
  - `messages` (ordered list)
  - `batch_size` (integer)
  - `accepted_at` (timestamp range)
- Relationships:
  - Produced from a mailbox dequeue cycle based on `BatchMailboxConfig`.
- Validation Rules:
  - `batch_size` MUST be between 1 and `max_batch_size`.
  - Message order MUST preserve mailbox ordering policy.

## Entity: BatchProcessingOutcome
- Fields:
  - `event_id` (uint64)
  - `actor_id` (string)
  - `batch_size` (integer)
  - `result` (enum: `batch_success`, `batch_failed_handler`, `batch_failed_supervision`)
  - `reason_code` (string, optional)
  - `timestamp` (timestamp)
- Relationships:
  - Emitted for each batch execution attempt.
- Validation Rules:
  - Every batch execution attempt emits exactly one batch outcome.
  - Failure outcomes include machine-readable `reason_code`.

## Entity: DownstreamBulkOperation
- Fields:
  - `actor_id` (string)
  - `operation_id` (string)
  - `message_count` (integer)
  - `result` (enum: `applied`, `failed`)
  - `completed_at` (timestamp)
- Relationships:
  - Triggered by batching-enabled actor behavior using `BatchEnvelope`.
- Validation Rules:
  - `message_count` MUST equal number of messages included in actor-issued bulk call.
  - Operation records MUST remain attributable to source actor.

## State Transitions

### Batch Configuration
1. `disabled -> enabled` when actor opts into batching with valid batch size.
2. `enabled -> disabled` when actor reverts to single-message processing.

### Batch Retrieval Cycle
1. `ready -> batch_success` when runtime retrieves 1..N messages and actor processing succeeds.
2. `ready -> batch_failed_handler` when actor batch handling fails.
3. `batch_failed_handler -> batch_failed_supervision` when supervisor decision transitions actor state (restart/stop/escalate).

### Downstream Optimization
1. `batch_success -> applied` when actor issues one grouped downstream operation.
2. `batch_success -> failed` when grouped downstream operation fails and actor-level error handling path is activated.
