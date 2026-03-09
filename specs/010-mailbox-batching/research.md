# Phase 0 Research: Smart Mailboxes and Message Batching

## Decision 1
- Decision: Introduce an opt-in batching mode per actor, defaulting to existing single-message behavior.
- Rationale: Preserves backward compatibility while enabling throughput improvements only where explicitly needed.
- Alternatives considered:
  - Global batching enabled for all actors: rejected because it could change semantics and latency characteristics unexpectedly.
  - Separate runtime for batched actors: rejected due to duplicated lifecycle/supervision logic.

## Decision 2
- Decision: Add bounded bulk dequeue per execution cycle (configured max batch size).
- Rationale: Prevents mailbox starvation and keeps scheduling fairness predictable.
- Alternatives considered:
  - Drain entire mailbox each cycle: rejected because large queues can monopolize actor execution.
  - Time-window batching only: rejected as more complex and less deterministic for tests.

## Decision 3
- Decision: Keep mailbox ordering guarantees intact within each actor batch.
- Rationale: Existing actor semantics rely on predictable ordering; batching should optimize retrieval, not reorder behavior.
- Alternatives considered:
  - Reorder by message type/priority inside a batch: rejected as a semantic change outside feature scope.

## Decision 4
- Decision: Support batch-specific outcome telemetry (batch size, success/failure class, reason code) while retaining existing per-message outcomes where required.
- Rationale: Enables verification of batching behavior, operational diagnostics, and regression prevention.
- Alternatives considered:
  - Metrics-only visibility: rejected because deterministic contract validation requires explicit outcomes.

## Decision 5
- Decision: Model bulk I/O optimization at actor-behavior level (consumer can aggregate batch into one downstream operation).
- Rationale: Runtime should provide batch retrieval primitives without hardcoding downstream destination semantics.
- Alternatives considered:
  - Runtime-managed generic bulk I/O sink: rejected because it couples runtime to domain-specific storage behavior.

## Decision 6
- Decision: Route batch handler failures through existing supervisor policy decisions.
- Rationale: Maintains let-it-crash consistency and avoids introducing a second failure model.
- Alternatives considered:
  - Partial-batch retry loops inside runtime: rejected for additional complexity and ambiguous actor-level semantics.

## Decision 7
- Decision: Keep request-response, lifecycle hooks, and PID/location-transparency contracts unchanged while batching is enabled.
- Rationale: Feature must remain composable with existing architecture and future distributed evolution.
- Alternatives considered:
  - Dedicated batch-only actor interface with different addressing contract: rejected because it fragments public APIs.
