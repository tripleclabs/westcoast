# Implementation Plan: Supervisor Trees & Fault Tolerance

**Branch**: `004-supervisor-fault-tolerance` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/spec.md)
**Input**: Feature specification from `/specs/004-supervisor-fault-tolerance/spec.md`

## Summary

Implement supervisor-driven fault tolerance for actor panics using explicit let-it-crash semantics:
panic interception per actor, deterministic supervision decisions, clean state restarts, and mailbox
preservation of unprocessed messages across restart while keeping location-transparent caller contracts.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`)  
**Storage**: In-memory runtime state and mailbox queues only (no persistence in this feature)  
**Testing**: `go test` unit/integration/contract suites plus focused panic/restart/mailbox scenarios  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Panic interception overhead remains bounded; actor recovery path completes without runtime outage; retained mailbox messages continue at current throughput envelope  
**Constraints**: Panic isolation MUST be actor-local, restart MUST reset state to initial snapshot, queued-unprocessed mailbox messages MUST survive restart, caller-facing actor identity contracts remain stable  
**Scale/Scope**: Single-node supervision trees and actor lifecycle recovery behavior only; distributed supervision and remote transport are out of scope

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
specs/004-supervisor-fault-tolerance/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── supervision-fault-tolerance-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── mailbox.go
│   ├── supervisor.go
│   ├── state.go
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

**Structure Decision**: Extend existing actor runtime and supervision primitives in `src/actor` and
validate behavior through contract/integration/unit tests without introducing new top-level modules.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/research.md). All unknowns are resolved.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/contracts/supervision-fault-tolerance-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/contracts/supervision-fault-tolerance-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native fault tolerance design preserved; no schema-first runtime dependency added.
- [x] Actor panic isolation, supervision decisions, and recovery flows are explicitly specified.
- [x] Test-first execution order is captured in quickstart scenarios.
- [x] Caller-facing identity/addressing contracts remain location-transparent during restart cycles.
- [x] Recovery and mailbox-preservation behavior are measurable via deterministic test outcomes.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
