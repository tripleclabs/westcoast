# Implementation Plan: Actor Routers and Worker Pools

**Branch**: `009-actor-router-pools` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/spec.md)
**Input**: Feature specification from `/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/spec.md`

## Summary

Add router actor capabilities that dispatch messages across worker pools using stateless strategies (round-robin/random) and consistent-hash sharding so key-scoped workloads can parallelize safely while preserving location-transparent sender contracts.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`, `hash/fnv`)  
**Storage**: In-memory router configuration, pool membership, routing counters, and routing outcomes only (no persistence)  
**Testing**: `go test` with unit, integration, and contract suites; failing tests authored before implementation  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Routing overhead remains low relative to current local send path; no sender-side blocking introduced by router coordination under normal load  
**Constraints**: Router-facing contracts remain location-transparent; deterministic routing guarantees for round-robin and hash-key workloads; explicit failure outcomes for zero-worker and invalid-key dispatch  
**Scale/Scope**: Single-node router + worker pool behavior only; no distributed transport execution in this feature

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
specs/009-actor-router-pools/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── router-worker-pool-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── actor_ref.go
│   ├── types.go
│   ├── events.go
│   ├── outcome.go
│   └── routing.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Extend the existing runtime and routing primitives under `src/actor` to model router pools and shard dispatch, with behavior verification across contract/integration/unit suites.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/research.md). All technical context items are resolved with no open clarifications.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/contracts/router-worker-pool-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/contracts/router-worker-pool-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/quickstart.md)

## Post-Design Constitution Check

- [x] Router and pool behavior stays Go-native and in-memory without schema-heavy dependencies.
- [x] Router and worker failure outcomes are explicit and supervision-compatible.
- [x] Test-first strategy is captured for stateless, hash-shard, and failure routing flows.
- [x] Sender contracts remain location-transparent through logical router addressing.
- [x] Strategy implementations are simple, measurable, and bounded in coordination overhead.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
