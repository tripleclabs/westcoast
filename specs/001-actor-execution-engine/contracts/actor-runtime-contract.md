# Contract: Actor Runtime Public Interface

## Purpose
Defines the public behavior contract for clients interacting with the single-node Actor Execution
Engine.

## Public Operations

### 1. Create Actor
- Input:
  - `actor_id` (required, unique)
  - `initial_state` (required)
- Output:
  - Success: actor created and enter `starting`
  - Failure: `duplicate_actor_id`
- Guarantees:
  - No external caller can read/write state directly after creation.

### 2. Send Message
- Input:
  - `actor_id` (required)
  - `message_payload` (required)
- Output:
  - `accepted`
  - `rejected_full`
  - `rejected_stopped`
  - `rejected_not_found`
- Guarantees:
  - Call returns without waiting for target message execution.
  - For accepted messages, per-actor processing order equals acceptance order.

### 3. Stop Actor
- Input:
  - `actor_id` (required)
- Output:
  - `stopped`
  - `already_stopped`
  - `not_found`
- Guarantees:
  - Subsequent sends return non-accepted outcome.

### 4. Query Actor Status
- Input:
  - `actor_id` (required)
- Output:
  - `starting|running|restarting|stopped|not_found`
- Guarantees:
  - Status reflects lifecycle state machine defined in data model.

## Event Contract

### Runtime Event Types
- `actor_started`
- `actor_stopped`
- `actor_failed`
- `actor_restarted`
- `message_processed`
- `message_rejected`

### Event Fields
- `event_id`
- `event_type`
- `actor_id`
- `message_id` (optional)
- `timestamp`
- `result` (optional)
- `error_code` (optional)

## Invariants
- Actor state mutation occurs only during that actor's message handling.
- Failures in one actor do not terminate unrelated actors.
- Public addressing is actor ID based and does not expose process-bound references.
