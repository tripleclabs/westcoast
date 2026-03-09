# Phase 0 Research: Local Actor Registry & Discovery

## Decision 1
- Decision: Maintain an in-memory name-to-PID directory with explicit register, lookup, and unregister operations.
- Rationale: Meets local discovery requirements with minimal overhead and deterministic semantics.
- Alternatives considered:
  - External service discovery backend: rejected as out of scope and unnecessary for single-node runtime.
  - Hardcoded references only: rejected due to lack of dynamic discovery.

## Decision 2
- Decision: Enforce strict uniqueness for active name registrations with deterministic duplicate rejection.
- Rationale: Prevents ambiguous routing and keeps lookup outcomes predictable.
- Alternatives considered:
  - Last-write-wins replacement: rejected due to hidden ownership changes.
  - Multiple actors per name: rejected due to lookup ambiguity.

## Decision 3
- Decision: Use explicit not-found outcomes for missing lookups rather than fallback behavior.
- Rationale: Improves caller clarity and testability.
- Alternatives considered:
  - Silent nil lookup semantics: rejected due to weak error signaling.
  - Automatic actor creation on miss: rejected as scope expansion.

## Decision 4
- Decision: Bind registry cleanup to terminal actor lifecycle events (graceful stop and permanent supervision stop/escalate).
- Rationale: Prevents stale registry entries while preserving active actors through temporary restarts.
- Alternatives considered:
  - Time-based expiration only: rejected due to stale-window inconsistency.
  - Remove on any failure event: rejected because transient restarts would break valid routing.

## Decision 5
- Decision: Keep register/lookup/unregister operations concurrency-safe with deterministic outcomes under races.
- Rationale: Registry is shared runtime infrastructure and must remain correct under parallel actor activity.
- Alternatives considered:
  - Unsynchronized map access: rejected due to race and safety violations.
  - Global coarse lock around runtime operations: rejected due to excessive contention risk.

## Decision 6
- Decision: Emit observable registry operation outcomes (success/reject/not-found/lifecycle-unregister).
- Rationale: Required for contract-level validation and operational diagnostics.
- Alternatives considered:
  - Logs only: rejected due to weak structured assertion capability.
  - Metrics only: rejected due to missing per-operation causality.
