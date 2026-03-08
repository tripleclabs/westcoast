# Phase 0 Research: Actor Execution Engine

## Decision 1: Actor Addressing Contract
- Decision: Use opaque logical Actor IDs as the only public actor handle.
- Rationale: Ensures location transparency and prevents direct process-bound coupling.
- Alternatives considered:
  - Exposing in-memory references directly: rejected because it blocks distributed evolution.
  - Using caller-managed IDs: rejected because it weakens lifecycle ownership guarantees.

## Decision 2: Mailbox Backpressure Policy
- Decision: Use bounded per-actor mailboxes with non-blocking submit result codes
  (`accepted`, `rejected_full`, `rejected_stopped`).
- Rationale: Preserves producer responsiveness and makes overload behavior explicit/testable.
- Alternatives considered:
  - Unbounded queues: rejected due to unbounded memory risk.
  - Blocking enqueue: rejected because it violates asynchronous producer requirements.

## Decision 3: Sequential Processing Model
- Decision: Each actor processes exactly one message at a time from its own mailbox.
- Rationale: Guarantees deterministic per-actor ordering and strict state isolation boundaries.
- Alternatives considered:
  - Parallel handler execution per actor: rejected due to state race risks.
  - Global worker pool with shared handler state: rejected due to weakened isolation semantics.

## Decision 4: Failure Isolation and Recovery Surface
- Decision: Runtime returns structured processing outcomes and emits lifecycle events that
  supervisors consume to apply restart/stop/escalate policies.
- Rationale: Aligns with let-it-crash behavior while keeping recovery policy explicit.
- Alternatives considered:
  - Swallowing handler panics and continuing silently: rejected due to hidden data corruption risk.
  - Immediate node-level abort on actor failure: rejected due to poor fault isolation.

## Decision 5: Performance Validation Strategy
- Decision: Validate scale with benchmark profiles that exercise actor creation, enqueue,
  processing, and restart paths at high concurrency.
- Rationale: Confirms runtime behavior under realistic load and enforces measurable outcomes.
- Alternatives considered:
  - Microbenchmarks only: rejected because they miss system-level contention.
  - Manual ad-hoc testing: rejected because results are not repeatable.

## Decision 6: Observability Baseline
- Decision: Track actor restarts, mailbox depth, enqueue latency, and processing latency via
  pluggable metrics hooks and structured events.
- Rationale: Required by constitution and needed to debug failure/recovery behavior at scale.
- Alternatives considered:
  - Logs only: rejected because high-volume systems need aggregated metrics.
  - Metrics only without event detail: rejected because incident triage requires context.
