# Contract: PID Resolver and Delivery Interface

## Purpose
Define caller-visible and runtime-visible behavior for location-transparent PID issuance,
resolution, and message delivery outcomes.

## Canonical PID Shape
- `namespace` (string)
- `actor_id` (string)
- `generation` (int)

Contract guarantee:
- PID is opaque to callers; callers pass it as a value and do not mutate its fields.

## Public Operations

### 1. Issue PID
- Input:
  - `namespace`
  - `actor_id`
- Output:
  - PID with `generation=1`
- Failure:
  - `rejected_not_found` if actor identity cannot be issued in namespace.

### 2. Resolve PID
- Input:
  - PID
- Output:
  - `reachable` route metadata or resolver state outcome
- Closed outcomes:
  - `delivered`
  - `unresolved`
  - `rejected_stopped`
  - `rejected_stale_generation`
  - `rejected_not_found`

### 3. Deliver by PID
- Input:
  - PID
  - message payload
- Output:
  - One closed outcome from the contract set
- Guarantees:
  - Resolver validates generation before delivery.
  - Stale generation is never delivered.

### 4. Lifecycle Update
- Input:
  - actor lifecycle event (`started`, `restarting`, `stopped`)
- Output:
  - resolver state update and optional generation increment
- Guarantees:
  - Restart transition increments generation before reachable delivery resumes.

## Non-Functional Contract
- Resolver lookup p95 latency MUST be <= 25 µs on agreed single-node baseline profile.

## Event Contract
- `pid_resolved`
- `pid_unresolved`
- `pid_rejected`
- `pid_delivered`

Event fields:
- `event_id`
- `namespace`
- `actor_id`
- `generation`
- `outcome`
- `timestamp`

## Invariants
- Callers never depend on process-bound references.
- Every delivery attempt returns exactly one closed outcome.
- Generation mismatch always yields `rejected_stale_generation`.
