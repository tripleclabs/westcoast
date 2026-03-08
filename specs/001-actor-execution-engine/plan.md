# Implementation Plan: Actor Execution Engine

**Branch**: `001-actor-execution-engine` | **Date**: 2026-03-08 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/spec.md)
**Input**: Feature specification from `/specs/001-actor-execution-engine/spec.md`

## Summary

Build the single-node Actor Execution Engine that provides strict actor state isolation,
non-blocking message submission, sequential mailbox processing, and high actor density targets.
The design uses logical actor IDs for location transparency and explicit failure isolation with
supervision-ready lifecycle semantics.

## Technical Context

**Language/Version**: Go 1.24+
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `runtime`, `time`)
**Storage**: In-memory runtime state only (no persistence in this feature)
**Testing**: `go test` with table-driven unit tests, race-enabled integration tests, benchmark tests
**Target Platform**: Linux/macOS single-node runtime
**Project Type**: Runtime library
**Performance Goals**: Support >=1,000,000 active actors; >=99% enqueue operations complete in <=50 ms
**Constraints**: Per-actor sequential processing, non-blocking enqueue semantics, low-allocation hot paths,
logical actor addressing, isolated actor failure domains
**Scale/Scope**: One node; actor lifecycle, mailbox engine, dispatch loop, runtime metrics hooks

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
specs/001-actor-execution-engine/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── actor-runtime-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── actor_ref.go
│   ├── mailbox.go
│   ├── registry.go
│   └── supervisor.go
└── internal/
    └── metrics/

tests/
├── unit/
├── integration/
└── benchmark/
```

**Structure Decision**: Single runtime library with actor primitives in `src/actor` and dedicated
unit/integration/benchmark tests to enforce TDD and performance gates.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/research.md).
All technical unknowns from the template are resolved.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/contracts/actor-runtime-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/contracts/actor-runtime-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native design preserved in contracts and data model.
- [x] Failure isolation and supervision hooks modeled explicitly.
- [x] TDD-first flow documented in quickstart.
- [x] Logical actor IDs are the only public addressing primitive.
- [x] Performance goals and benchmark gates are explicit and testable.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
