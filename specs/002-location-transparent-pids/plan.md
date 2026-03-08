# Implementation Plan: Location-Transparent PIDs

**Branch**: `002-location-transparent-pids` | **Date**: 2026-03-08 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/spec.md)
**Input**: Feature specification from `/specs/002-location-transparent-pids/spec.md`

## Summary

Introduce a location-transparent PID model for the actor runtime so all caller-facing interactions
use resolver-based process identifiers instead of direct runtime references. The feature enforces a
stable logical PID with generation semantics, deterministic closed delivery outcomes, and a strict
single-node resolver lookup latency target.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`)  
**Storage**: In-memory resolver index and PID state only (no persistence in this feature)  
**Testing**: `go test` unit/integration/benchmark suites with dedicated PID contract and latency tests  
**Target Platform**: Linux/macOS single-node runtime
**Project Type**: Runtime library  
**Performance Goals**: PID resolver lookup p95 <= 25 µs on baseline single-node profile  
**Constraints**: Canonical PID shape (`namespace + actor_id + generation`), closed delivery outcomes,
resolver-validated stale generation rejection, no process-bound references in caller API  
**Scale/Scope**: Single-node PID issuance/resolution, lifecycle-aware routing semantics,
forward-compatible transport abstraction seams

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
specs/002-location-transparent-pids/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── pid-resolver-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── pid.go
│   ├── pid_resolver.go
│   ├── actor_ref.go
│   ├── runtime.go
│   └── events.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
├── unit/
└── benchmark/
```

**Structure Decision**: Extend the existing `src/actor` runtime package with dedicated PID and
resolver components while preserving current runtime and metrics organization.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/research.md). All technical unknowns are resolved.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/contracts/pid-resolver-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/contracts/pid-resolver-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native design preserved in PID modeling and resolver contract.
- [x] Failure isolation and supervision outcomes modeled for PID lifecycle transitions.
- [x] TDD-first implementation path documented in quickstart validation flow.
- [x] Logical identity and resolver boundaries enforce location transparency.
- [x] Performance gate is measurable (`p95 <= 25 µs`) and benchmarked.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
