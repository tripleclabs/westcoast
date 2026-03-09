# Phase 0 Research: Native Pub/Sub Event Bus

## Decision 1
- Decision: Implement broker as a first-class runtime actor with in-memory subscription state.
- Rationale: Satisfies zero-dependency requirement while keeping behavior consistent with existing actor lifecycle/supervision patterns.
- Alternatives considered:
  - External MQ dependency (NATS/Redis): rejected due to explicit requirement for built-in local broker.
  - Non-actor global singleton broker: rejected because it bypasses existing actor fault/isolation model.

## Decision 2
- Decision: Use a segment-based Trie for topic pattern indexing and match traversal.
- Rationale: Efficiently supports exact, single-segment wildcard, and multi-segment tail wildcard evaluation.
- Alternatives considered:
  - Linear scan over all patterns: rejected for poor scaling as subscription count grows.
  - Regex-per-subscription matching: rejected for higher per-publish overhead and less deterministic matching semantics.

## Decision 3
- Decision: Restrict wildcard grammar to `+` (single segment) and `#` (tail wildcard only).
- Rationale: Keeps matching deterministic and aligned with stated requirements.
- Alternatives considered:
  - Generalized wildcard operators: rejected as scope expansion and ambiguity risk.

## Decision 4
- Decision: Manage subscribe/unsubscribe via Ask command envelopes with deterministic acknowledgements.
- Rationale: Reuses existing request-response semantics and supports runtime dynamic updates safely.
- Alternatives considered:
  - Fire-and-forget management commands: rejected because callers need confirmation and deterministic outcomes.

## Decision 5
- Decision: Publish fan-out performs asynchronous per-target send attempts while collecting explicit delivery outcomes.
- Rationale: Preserves non-blocking publish behavior and operational visibility across partial delivery failures.
- Alternatives considered:
  - Synchronous serial delivery: rejected due to higher publish latency and tighter coupling to slow subscribers.

## Decision 6
- Decision: Enforce idempotent subscription inserts for duplicate PID+pattern combinations.
- Rationale: Simplifies runtime behavior and prevents duplicate deliveries from repeated subscribe requests.
- Alternatives considered:
  - Allow duplicates and de-duplicate at publish time: rejected due to avoidable hot-path overhead.

## Decision 7
- Decision: Keep broker contracts location-transparent by requiring logical broker actor identity and PID-based delivery targets.
- Rationale: Maintains compatibility with future multi-node transport evolution.
- Alternatives considered:
  - Expose process-local subscriber references: rejected for violating location transparency principle.
