# Contract: Local Actor Registry & Discovery

## Purpose
Define caller-visible guarantees for named actor registration, dynamic lookup, and lifecycle-synced
registry cleanup in the local runtime.

## Registration Contract
- A running actor MAY register a human-readable name.
- Names MUST be validated and empty/invalid names rejected.
- Active names MUST be unique; duplicate registration attempts are rejected deterministically.

## Lookup Contract
- Lookup by registered name returns the corresponding PID.
- Lookup by unknown name returns deterministic `not_found` outcome.
- Lookup does not mutate registry state.

## Lifecycle Sync Contract
- Registry entries MUST be removed when actors are gracefully stopped.
- Registry entries MUST be removed when actors are permanently terminated by supervision policy.
- Temporary restart paths MUST not remove active registration unless actor becomes terminally unavailable.

## Concurrency Contract
- Concurrent register/lookup/unregister operations must remain deterministic and race-safe.
- Duplicate race resolution must produce one successful owner and deterministic rejection for others.

## Location Transparency Contract
- Discovery interfaces expose logical names and PIDs, not process pointers.
- Caller contracts remain compatible with future multi-node routing transport.

## Event/Outcome Contract
Required registry operation outcomes:
- `register_success`
- `register_rejected_duplicate`
- `lookup_hit`
- `lookup_not_found`
- `unregister_success`
- `unregister_lifecycle_terminal`

Required fields for operation telemetry:
- `event_id`
- `name`
- `actor_id` (if available)
- `pid` (if available)
- `result`
- `timestamp`

## Invariants
- At most one active mapping per name.
- No stale name->PID mappings after terminal actor stop.
- Lookup semantics are deterministic under concurrent access.
