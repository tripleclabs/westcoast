# Data Model: Location-Transparent PIDs

## Entity: PID
- Fields:
  - `namespace` (string, required)
  - `actor_id` (string, required)
  - `generation` (integer >= 1)
- Relationships:
  - Identifies one logical actor identity and one lifecycle incarnation.
- Validation Rules:
  - `namespace + actor_id + generation` MUST be unique in resolver index.
  - `generation` MUST increment on supervised restart.

## Entity: PIDResolverEntry
- Fields:
  - `pid` (PID, unique key)
  - `route_state` (enum: `reachable`, `unresolved`, `stopped`, `restarting`)
  - `current_generation` (integer >= 1)
  - `updated_at` (timestamp)
- Relationships:
  - Maps one PID identity to one runtime actor route at a point in time.
- Validation Rules:
  - Delivery request generation MUST match `current_generation` for `reachable` state.
  - Mismatch MUST produce `rejected_stale_generation`.

## Entity: PIDDeliveryOutcome
- Fields:
  - `pid` (PID)
  - `outcome` (enum: `delivered`, `unresolved`, `rejected_stopped`, `rejected_stale_generation`, `rejected_not_found`)
  - `emitted_at` (timestamp)
  - `error_code` (optional string)
- Relationships:
  - Produced by one resolver+delivery attempt.
- Validation Rules:
  - Every PID message attempt MUST return exactly one closed-set outcome.

## Entity: PIDResolutionEvent
- Fields:
  - `event_id` (uint64, unique)
  - `pid` (PID)
  - `event_type` (enum: `pid_resolved`, `pid_unresolved`, `pid_rejected`, `pid_delivered`)
  - `outcome` (PIDDeliveryOutcome.outcome)
  - `timestamp` (timestamp)
- Relationships:
  - Emitted by resolver and delivery paths for observability.
- Validation Rules:
  - Events MUST use canonical outcome names.

## State Transitions

### PID Resolver Entry Lifecycle
1. `unresolved -> reachable` when actor is created and PID is registered.
2. `reachable -> restarting` when supervised restart begins.
3. `restarting -> reachable` when actor resumes with incremented generation.
4. `reachable -> stopped` when actor is terminated.
5. `stopped -> unresolved` when resolver entry is retired.

### PID Delivery Attempt
1. `requested -> delivered` when PID resolves and generation matches.
2. `requested -> unresolved` when resolver has no entry.
3. `requested -> rejected_not_found` when actor identity does not exist in namespace.
4. `requested -> rejected_stopped` when target actor route state is stopped.
5. `requested -> rejected_stale_generation` when generation validation fails.
