# Phase 0 Research: Type-Agnostic Local Messaging

## Decision 1
- Decision: Use native in-memory structured payload passing for intra-node delivery with no local encode/decode stage.
- Rationale: Meets feature goal and avoids serializer overhead on local hot path.
- Alternatives considered:
  - JSON/Protobuf for all paths: rejected due to unnecessary local overhead.
  - Reflection-heavy dynamic conversion between message envelopes: rejected due to complexity and allocation cost.

## Decision 2
- Decision: Apply exact-type-first routing with one optional explicit fallback branch.
- Rationale: Ensures deterministic behavior and avoids ambiguous handler precedence.
- Alternatives considered:
  - First-registered handler wins: rejected due to ordering fragility.
  - Deep compatibility matching: rejected due to complexity and unpredictable routing.

## Decision 3
- Decision: Enforce explicit rejection outcomes for invalid classes:
  `rejected_unsupported_type`, `rejected_nil_payload`, `rejected_version_mismatch`.
- Rationale: Enables deterministic contract tests and clearer incident diagnosis.
- Alternatives considered:
  - Silent drop for unsupported payloads: rejected due to observability gap.
  - Automatic conversion/upcasting: rejected due to hidden behavior changes.

## Decision 4
- Decision: Preserve transport-agnostic message contracts so distributed mode can introduce serialization later without changing local caller semantics.
- Rationale: Protects location transparency and reduces future migration risk.
- Alternatives considered:
  - Introduce distributed serialization concerns now: rejected as out of scope.
  - Expose local-only internal references to callers: rejected due to architecture lock-in.

## Decision 5
- Decision: Set measurable local performance gates:
  p95 local send latency <= 25 µs and throughput >= 1,000,000 msgs/sec on baseline profile.
- Rationale: Converts “high-performance” requirement into enforceable acceptance criteria.
- Alternatives considered:
  - Throughput-only gate: rejected as insufficient for latency-sensitive workloads.
  - No numeric gate: rejected as non-testable.

## Decision 6
- Decision: Emit structured message-routing events for success and rejection outcomes.
- Rationale: Required for debugging route mismatches and runtime behavior under load.
- Alternatives considered:
  - Logs only: rejected due to poor aggregation and contract testability.
  - Metrics only: rejected due to missing per-event diagnostic context.
