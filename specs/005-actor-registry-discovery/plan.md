# Implementation Plan: Local Actor Registry & Discovery

**Branch**: `005-actor-registry-discovery` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/spec.md)
**Input**: Feature specification from `/specs/005-actor-registry-discovery/spec.md`

## Summary

Implement a local actor registry that supports optional human-readable named registration, dynamic
name-to-PID lookup, and automatic lifecycle-synced cleanup when actors are permanently stopped.
The design preserves location-transparent caller contracts and deterministic concurrent registry semantics.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`sync`, `sync/atomic`, `time`, `context`)  
**Storage**: In-memory registry map structures in runtime scope only  
**Testing**: `go test` unit/integration/contract suites with concurrent register/lookup/remove cases  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Name lookup remains low-latency under concurrent access; lifecycle cleanup does not block actor stop path materially  
**Constraints**: One active name->PID mapping, deterministic duplicate handling, stale entry prevention after terminal actor stop, location-transparent contracts  
**Scale/Scope**: Single-node local registry/discovery only; cross-node discovery and distributed consensus out of scope

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- [x] Go-native design: No mandatory external schema-first serialization in runtime hot path.
- [x] Let-it-crash semantics: Failure domains and supervision strategy are explicit.
- [x] Test-first proof: Planned failing tests exist for each behavior change.
- [x] Location transparency: Public contracts use logical actor identity/address abstractions.
- [x] Performance simplicity: Hot-path allocations/dependencies are justified with metrics plan.

Gate Status (Pre-Research): PASS

## Project Structure

### Documentation (this feature)

```text
specs/005-actor-registry-discovery/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── actor-registry-discovery-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── registry.go
│   ├── runtime.go
│   ├── pid.go
│   ├── types.go
│   └── events.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Extend existing local actor registry/runtime packages in `src/actor` and
validate deterministic behavior using unit/integration/contract tests without adding external services.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/research.md). All unknowns are resolved.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/contracts/actor-registry-discovery-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/contracts/actor-registry-discovery-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native in-memory discovery design preserved; no schema-first serializer introduced.
- [x] Failure boundaries remain actor-local with deterministic terminal cleanup semantics.
- [x] Test-first execution order is captured in quickstart verification flow.
- [x] Named discovery contracts remain location-transparent and PID-based.
- [x] Registry hot-path simplicity and concurrency behavior are measurable and test-driven.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
