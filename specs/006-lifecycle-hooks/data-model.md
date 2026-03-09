# Data Model: Lifecycle Management Hooks

## Entity: LifecycleHookDefinition
- Fields:
  - `actor_id` (string, required)
  - `has_start_hook` (boolean)
  - `has_stop_hook` (boolean)
- Relationships:
  - One definition belongs to one actor instance.
- Validation Rules:
  - `actor_id` MUST be non-empty.
  - Hooks are optional, but definition MUST be valid even when both hooks are absent.

## Entity: ActorLifecycleState
- Fields:
  - `actor_id` (string)
  - `status` (enum: `starting`, `running`, `stopping`, `stopped`)
  - `updated_at` (timestamp)
- Relationships:
  - One active state per actor instance at a time.
- Validation Rules:
  - State transitions MUST follow allowed lifecycle order.
  - Actors with Start hook failures MUST NOT transition to `running`.

## Entity: LifecycleExecutionRecord
- Fields:
  - `actor_id` (string)
  - `phase` (enum: `start`, `stop`)
  - `result` (enum: `success`, `failed_error`, `failed_panic`)
  - `reason` (string, optional)
  - `executed_at` (timestamp)
- Relationships:
  - Multiple execution records can exist for one actor across restarts.
- Validation Rules:
  - Every hook execution MUST produce exactly one deterministic result.
  - Start phase record MUST exist before first message is processed when Start hook is configured.

## Entity: LifecycleOutcomeEvent
- Fields:
  - `event_id` (uint64)
  - `actor_id` (string)
  - `phase` (enum: `start`, `stop`)
  - `result` (enum: `start_success`, `start_failed`, `stop_success`, `stop_failed`)
  - `timestamp` (timestamp)
  - `error_code` (string, optional)
- Relationships:
  - Emitted from each lifecycle hook execution path.
- Validation Rules:
  - Outcome events MUST be location-transparent and actor identity based.
  - Failure events MUST include a machine-readable reason code.

## State Transitions

### Startup Path
1. `starting -> running` when Start hook succeeds or no Start hook is configured.
2. `starting -> stopped` when Start hook fails.

### Shutdown Path
1. `running -> stopping` when graceful shutdown is initiated.
2. `stopping -> stopped` after Stop hook attempt completes, regardless of success/failure.

### Restart Path
1. `stopped -> starting` when supervision restart decision is applied.
2. Start hook executes again for the new actor instance lifecycle.
