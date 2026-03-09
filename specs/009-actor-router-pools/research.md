# Phase 0 Research: Actor Routers and Worker Pools

## Decision 1
- Decision: Model router strategies as explicit runtime modes (`round_robin`, `random`, `consistent_hash`) bound to a logical router actor identity.
- Rationale: Keeps sender contracts stable while allowing routing behavior to vary without changing caller code.
- Alternatives considered:
  - Create separate router actor types per strategy: rejected due to duplicated surface area and harder migration between strategies.
  - Embed strategy logic in sender code: rejected because it breaks location transparency and central routing control.

## Decision 2
- Decision: Use atomic counters for round-robin worker selection under concurrent dispatch.
- Rationale: Preserves deterministic ordering with low overhead and avoids global lock contention on the hot path.
- Alternatives considered:
  - Mutex-guarded index updates: rejected as unnecessary contention overhead.
  - Per-sender counters: rejected because it weakens global fairness guarantees.

## Decision 3
- Decision: Use a standard hash function over `HashKey()` output to map stateful messages to worker indices.
- Rationale: Provides deterministic, repeatable shard assignment for stable worker membership.
- Alternatives considered:
  - Non-deterministic/random fallback for missing keys: rejected because it violates state-affinity guarantees.
  - Custom pluggable hash in this phase: rejected as unnecessary complexity for initial feature scope.

## Decision 4
- Decision: Require explicit deterministic failure outcomes for zero-worker pools, invalid hash-key messages, and unroutable worker targets.
- Rationale: Makes routing failures testable and operationally diagnosable.
- Alternatives considered:
  - Silent drop on failure: rejected because failures become unobservable.
  - Implicit fallback to first worker: rejected because it hides configuration errors.

## Decision 5
- Decision: Keep worker pool membership static per router configuration in this phase, with clear behavior when membership is replaced.
- Rationale: Supports deterministic sharding guarantees and keeps implementation bounded.
- Alternatives considered:
  - Auto-discovered dynamic worker sets: rejected as out of scope for single-node phase.
  - Live incremental membership changes with remap minimization now: rejected as premature complexity.

## Decision 6
- Decision: Preserve existing supervision semantics for router and worker actor failures.
- Rationale: Maintains constitutional alignment and avoids introducing a second fault model.
- Alternatives considered:
  - Router-specific bespoke supervision tree now: rejected because current supervisor primitives already satisfy feature needs.

## Decision 7
- Decision: Emit routing outcome telemetry for strategy selection, target worker, and failure reasons.
- Rationale: Enables contract validation, debugging, and readiness checks without coupling business logic to internals.
- Alternatives considered:
  - Metrics-only visibility: rejected because exact per-message outcomes are needed for deterministic tests.
