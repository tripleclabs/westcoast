# Contract: PID Gateway Guardrails

## Purpose
Define caller-visible and operator-visible guarantees for PID-only interaction policy and pluggable gateway boundary behavior.

## PID Interaction Policy Contract
- Cross-actor interactions MUST be expressed through PID contracts.
- Non-PID cross-actor interactions MUST be rejected deterministically.
- Policy acceptance and rejection outcomes MUST be observable.

## Gateway Boundary Contract
- Runtime MUST expose a gateway-boundary seam capable of routing standard PID messages.
- Business actor message semantics MUST remain unchanged across `local_direct` and `gateway_mediated` modes.
- If gateway routing cannot be resolved, runtime MUST emit deterministic routing failure outcome.

## Business Logic Transparency Contract
- Actor business logic MUST not require topology-aware branching.
- Public contracts MUST not expose node locality or transport implementation details.

## Supervision Compatibility Contract
- Failures during compliant PID message processing continue to use existing supervision decisions (restart/stop/escalate).
- Guardrail policy checks MUST not alter established supervision semantics.

## Observability Contract
Required outcomes:
- `policy_accept`
- `policy_reject_non_pid`
- `gateway_route_success`
- `gateway_route_failure`

Required fields:
- `event_id`
- `actor_id`
- `pid` (if available)
- `outcome`
- `timestamp`
- `reason_code` (for failures/rejections)

## Readiness Validation Contract
- Release-readiness checks MUST validate PID policy, gateway boundary, and location transparency scopes.
- A failed scope MUST produce explicit evidence and block guardrail readiness sign-off.

## Invariants
- No compliant cross-actor flow bypasses PID contracts.
- Gateway seam insertion does not require business actor logic rewrites.
- Guardrail outcomes remain deterministic across equivalent interactions.
- Release-readiness validation MUST report `pid_policy`, `gateway_boundary`, and `location_transparency` scopes each cycle.
