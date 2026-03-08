# Phase 0 Research: Location-Transparent PIDs

## Decision 1
- Decision: Define canonical PID as `namespace + actor_id + generation`.
- Rationale: Separates logical identity from lifecycle incarnation and preserves future routing seam.
- Alternatives considered:
  - `actor_id` only: rejected because restart and namespace boundaries become ambiguous.
  - `node_id + actor_id + generation`: rejected because node-first identity leaks deployment concerns to callers.

## Decision 2
- Decision: Keep logical PID stable while incrementing generation on supervised restart.
- Rationale: Preserves caller contract stability and prevents stale deliveries through generation validation.
- Alternatives considered:
  - New PID on every restart: rejected due to caller re-discovery churn.
  - No generation: rejected due to stale-message ambiguity.

## Decision 3
- Decision: Use strict closed delivery outcomes (`delivered`, `unresolved`, `rejected_stopped`, `rejected_stale_generation`, `rejected_not_found`).
- Rationale: Prevents ambiguous reason strings and simplifies deterministic contract testing.
- Alternatives considered:
  - Generic `rejected` with reason text: rejected due to weak testability.
  - Broad status families (`success/retryable/fatal`): rejected due to loss of domain precision.

## Decision 4
- Decision: Define resolver as an internal interface boundary that can later back onto distributed discovery/transport.
- Rationale: Keeps single-node implementation simple while preserving multi-node evolution path.
- Alternatives considered:
  - Hardcoding runtime map access in callers: rejected due to coupling and migration risk.
  - Implementing distributed transport now: rejected as out of scope and unnecessary complexity.

## Decision 5
- Decision: Enforce resolver lookup latency target p95 <= 25 µs on baseline single-node profile.
- Rationale: Aligns performance expectations with actor-system ergonomics and high-frequency messaging.
- Alternatives considered:
  - Millisecond-level SLOs: rejected as too slow for this domain.
  - p95 <= 10 µs: deferred as a future optimization target after baseline adoption.

## Decision 6
- Decision: Emit PID resolution and delivery events with standardized outcome labels.
- Rationale: Supports observability and operational diagnostics for failure/recovery paths.
- Alternatives considered:
  - Minimal logging only: rejected due to weak aggregation and alertability.
  - Metrics only without labeled outcomes: rejected because root-cause analysis needs event context.
