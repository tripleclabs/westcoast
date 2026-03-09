# Phase 0 Research: Request-Response Ask Semantics

## Decision 1
- Decision: Represent each Ask as a runtime-managed pending wait slot keyed by a unique correlation identifier, completed by the first valid reply.
- Rationale: Guarantees deterministic one-response completion and prevents duplicate-reply races from delivering multiple results.
- Alternatives considered:
  - Block on ad-hoc per-call channels without central correlation tracking: rejected due to weak duplicate/late-reply handling.
  - Reuse actor IDs as correlation key: rejected because concurrent Ask calls can collide.

## Decision 2
- Decision: Attach implicit Ask context to Ask-originated messages, including `ReplyTo` PID and correlation identifier.
- Rationale: Gives receivers explicit routing and correlation data without out-of-band contracts, aligning with location transparency.
- Alternatives considered:
  - Require payload-level caller-provided reply fields: rejected because it leaks protocol burden into business messages.
  - Require receiver-side lookup tables keyed by sender identity: rejected due to ambiguity under concurrency.

## Decision 3
- Decision: Use ephemeral, request-scoped ReplyTo PID targets for Ask responses.
- Rationale: Limits reply scope to a single request and isolates late/invalid reply behavior.
- Alternatives considered:
  - Shared long-lived reply inbox per caller: rejected because it increases demultiplexing complexity and risk of cross-wire bugs.
  - Direct callback closures: rejected because closure identity is not location-transparent.

## Decision 4
- Decision: Timeout and caller cancellation close Ask wait slots deterministically; late replies after closure are recorded and ignored for completion.
- Rationale: Prevents resource leaks and ensures bounded wait behavior for callers.
- Alternatives considered:
  - Keep wait slot alive until explicit cleanup: rejected because it risks unbounded memory growth.
  - Deliver late replies as successful completion after timeout: rejected because it violates timeout contract determinism.

## Decision 5
- Decision: Permit responders to reply asynchronously from delegated background work using only the supplied ReplyTo PID.
- Rationale: Keeps actor event loop unblocked while preserving explicit reply routing semantics.
- Alternatives considered:
  - Require replies from within actor handler frame only: rejected because it blocks throughput for long-running work.
  - Add special worker actor requirement for every deferred reply: rejected as unnecessary complexity for base Ask semantics.

## Decision 6
- Decision: Emit explicit Ask lifecycle outcomes for success, timeout, invalid-reply-target, and late-reply-drop cases.
- Rationale: Supports contract testing, production diagnostics, and readiness verification.
- Alternatives considered:
  - Log-only signaling: rejected due to weaker deterministic assertions in tests.
  - Silent late-reply drops: rejected because operability and debugging would degrade.

## Decision 7
- Decision: Keep Ask semantics within current single-node runtime while preserving PID-only interfaces for future gateway-mediated delivery.
- Rationale: Delivers immediate feature value without pre-optimizing distributed transport internals.
- Alternatives considered:
  - Implement cross-node Ask transport now: rejected as out of scope for this feature.
  - Use non-PID direct references for replies: rejected because it breaks location transparency guarantees.
