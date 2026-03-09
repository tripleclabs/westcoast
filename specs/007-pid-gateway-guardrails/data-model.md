# Data Model: Distributed-Ready Guardrails

## Entity: PIDInteractionPolicy
- Fields:
  - `policy_id` (string)
  - `cross_actor_mode` (enum: `pid_only`)
  - `enforcement_state` (enum: `active`, `inactive`)
  - `updated_at` (timestamp)
- Relationships:
  - One active policy applies across runtime interaction boundaries.
- Validation Rules:
  - `cross_actor_mode` MUST be `pid_only` when enforcement is active.
  - Policy transitions MUST be explicit and observable.

## Entity: GatewayBoundaryRoute
- Fields:
  - `route_id` (string)
  - `delivery_mode` (enum: `local_direct`, `gateway_mediated`)
  - `pid_namespace` (string)
  - `target_actor_id` (string)
  - `status` (enum: `routable`, `not_routable`)
- Relationships:
  - Links PID-targeted interactions to current delivery mode.
- Validation Rules:
  - Route decisions MUST preserve standard PID message contract shape.
  - `gateway_mediated` mode MUST not expose topology details to business actors.

## Entity: GuardrailOutcome
- Fields:
  - `event_id` (uint64)
  - `policy_id` (string)
  - `actor_id` (string)
  - `pid` (PID, optional for rejected pre-routing attempts)
  - `outcome` (enum: `policy_accept`, `policy_reject_non_pid`, `gateway_route_success`, `gateway_route_failure`)
  - `reason_code` (string, optional)
  - `timestamp` (timestamp)
- Relationships:
  - Produced by policy and gateway-boundary decisions.
- Validation Rules:
  - Every cross-actor routing decision MUST emit exactly one deterministic outcome.
  - Rejection/failure outcomes MUST include a machine-readable reason.

## Entity: ReadinessValidationRecord
- Fields:
  - `validation_id` (string)
  - `scope` (enum: `pid_policy`, `gateway_boundary`, `location_transparency`)
  - `result` (enum: `pass`, `fail`)
  - `checked_at` (timestamp)
  - `evidence_ref` (string)
- Relationships:
  - Aggregates guardrail checks for release-readiness review.
- Validation Rules:
  - Each release readiness cycle MUST include all three scopes.
  - Any `fail` result MUST block readiness sign-off until resolved.

## State Transitions

### Policy Enforcement
1. `inactive -> active` when PID-only policy is enabled.
2. `active -> active` for compliant PID interactions.
3. `active -> policy_reject_non_pid` for non-compliant attempts.

### Gateway Boundary Routing
1. `local_direct` mode routes PID messages directly.
2. `gateway_mediated` mode routes PID messages through gateway seam.
3. Routing failure emits deterministic `gateway_route_failure` without mutating business actor contracts.

### Readiness Validation
1. `unverified -> pass` when all guardrail scopes validate.
2. `unverified -> fail` when any scope fails.
3. `fail -> pass` only after failed scope evidence is corrected and revalidated.
