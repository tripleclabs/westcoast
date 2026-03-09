# Phase 0 Research: Distributed-Ready Guardrails

## Decision 1
- Decision: Enforce PID-only policy for cross-actor interactions at runtime boundaries.
- Rationale: Guarantees caller behavior remains transport-agnostic and prevents direct actor coupling that blocks multi-node evolution.
- Alternatives considered:
  - Allow mixed actor-id and PID cross-actor calls: rejected because it weakens migration guarantees and introduces hidden coupling.
  - Apply policy only through code review guidance: rejected due to lack of deterministic runtime enforcement.

## Decision 2
- Decision: Define a gateway-boundary seam that accepts standard PID delivery contracts and can be switched between local direct and gateway-mediated routing.
- Rationale: Preserves current single-node behavior while creating a clean insertion point for future network transport.
- Alternatives considered:
  - Hardwire local direct routing with no seam: rejected due to rewrite risk when introducing distributed transport.
  - Introduce full network transport now: rejected as out-of-scope for phase-2 prep.

## Decision 3
- Decision: Keep business actor handlers unaware of route topology and gateway mediation details.
- Rationale: Ensures domain logic portability and prevents topology leakage into actor behavior.
- Alternatives considered:
  - Expose route source details directly to actor handlers: rejected because it couples business logic to transport shape.
  - Require separate distributed-aware actor interfaces: rejected because it fragments the actor programming model.

## Decision 4
- Decision: Emit explicit policy/gateway outcomes for accepted/rejected/routing-failure flows.
- Rationale: Enables validation, release-readiness checks, and deterministic diagnosis of guardrail regressions.
- Alternatives considered:
  - Logs-only evidence: rejected due to weaker testability and inconsistent automated verification.
  - Silent fallback behavior: rejected because architecture drift becomes hard to detect.

## Decision 5
- Decision: Preserve existing supervision semantics for compliant PID flow failures.
- Rationale: Maintains constitution-aligned fault boundaries and avoids introducing new failure semantics during guardrail adoption.
- Alternatives considered:
  - Add new supervision branches specifically for gateway-policy failures now: rejected as unnecessary complexity for this phase.

## Decision 6
- Decision: Keep policy and gateway-boundary state in memory for this phase.
- Rationale: Current runtime scope is local single-node, and persistence does not add value for architectural guardrails.
- Alternatives considered:
  - Persist compliance decisions: rejected as operational overhead without required product value.
