# Quickstart: Validate Distributed-Ready Guardrails

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestPIDOnlyPolicyRejectsNonPIDCrossActorInteraction
go test ./tests/integration/... -run TestGatewayBoundaryModeKeepsBusinessLogicUnchanged
go test ./tests/integration/... -run TestReadinessValidationFailThenPass
go test ./tests/contract/... -run TestPIDGatewayGuardrailsContract
```

Expected: tests fail before implementation changes.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- PID-only cross-actor interaction guardrails
- Pluggable gateway-boundary seam for PID delivery
- Deterministic policy and gateway outcomes
- Readiness validation signals for architecture review

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestPIDOnlyPolicyRejectsNonPIDCrossActorInteraction|TestPIDCompliantFlowRemainsBehaviorallyEquivalent|TestGatewayBoundaryModeKeepsBusinessLogicUnchanged|TestGatewayBoundaryRoutingFailureIsDeterministic'
```

Pass criteria:
- Cross-actor non-PID interactions are rejected deterministically.
- PID-compliant interactions preserve current business behavior.
- Gateway boundary mode does not leak topology concerns into business logic.
- Policy and gateway outcomes are observable and deterministic.

## 5. Constitution Alignment Checks
- Go-native actor runtime remains intact.
- Fault handling and supervision semantics remain explicit.
- Failing-then-passing tests demonstrate behavior changes.
- Public contracts remain location-transparent.
- Guardrail enforcement overhead remains simple and measurable.

## 6. Expected Outcomes
- Runtime can evolve to multi-node routing without business actor rewrites.
- PID contracts are consistently enforced at cross-actor boundaries.
- Architecture reviewers can validate distributed-readiness guardrails with explicit evidence.
