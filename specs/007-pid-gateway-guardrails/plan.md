# Implementation Plan: Distributed-Ready Guardrails

**Branch**: `007-pid-gateway-guardrails` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/spec.md)
**Input**: Feature specification from `/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/spec.md`

## Summary

Establish architecture guardrails that enforce PID-only cross-actor interactions and define a pluggable gateway boundary so future multi-node transport can be introduced without rewriting business actor logic.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`)  
**Storage**: In-memory policy validation state and observability outcomes only (no persistence)  
**Testing**: `go test` with unit, integration, and contract suites; failing tests authored before implementation  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: PID-policy enforcement and gateway-seam checks add minimal overhead to message dispatch paths  
**Constraints**: Cross-actor interactions constrained to PID contracts; pluggable gateway seam must preserve business-logic transparency; no distributed transport implementation in this phase  
**Scale/Scope**: Single-node runtime guardrails and seams only; multi-node transport execution is out of scope

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] Go-native design: No mandatory external schema-first serialization in runtime hot path.
- [x] Let-it-crash semantics: Failure domains and supervision strategy are explicit.
- [x] Test-first proof: Planned failing tests exist for each behavior change.
- [x] Location transparency: Public contracts use logical actor identity/address abstractions.
- [x] Performance simplicity: Hot-path allocations/dependencies are justified with metrics plan.

## Project Structure

### Documentation (this feature)

```text
specs/007-pid-gateway-guardrails/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── pid-gateway-guardrails-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── pid.go
│   ├── pid_resolver.go
│   ├── types.go
│   ├── events.go
│   └── outcome.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Extend existing runtime PID and dispatch components in `src/actor` with policy and gateway-boundary seams, validated via existing test suite partitions.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/research.md). All technical context items are resolved with no open clarifications.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/contracts/pid-gateway-guardrails-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/contracts/pid-gateway-guardrails-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native runtime guardrails preserve idiomatic in-memory actor messaging behavior.
- [x] PID-policy and gateway-boundary failures remain explicit with deterministic outcomes and supervision boundaries.
- [x] Test-first validation flow is captured across policy, contract, and integration scenarios.
- [x] Public addressing remains location-transparent and ready for future transport insertion.
- [x] Gateway seam and policy checks are minimal and measurable, avoiding unnecessary abstraction overhead.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
