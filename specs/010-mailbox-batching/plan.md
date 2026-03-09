# Implementation Plan: Smart Mailboxes and Message Batching

**Branch**: `010-mailbox-batching` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/spec.md)
**Input**: Feature specification from `/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/spec.md`

## Summary

Add opt-in mailbox batching so actors can consume multiple queued messages per execution cycle, enabling high-throughput processing and efficient downstream bulk operations while preserving supervision semantics and location-transparent caller contracts.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`)  
**Storage**: In-memory mailbox queues, actor runtime state, and batching outcomes only (no persistence)  
**Testing**: `go test` unit/integration/contract suites with failing tests authored before implementation  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Increase effective message throughput for eligible actors; reduce downstream operation count for I/O sink actors under burst load; maintain bounded per-cycle processing and predictable latency  
**Constraints**: Batching must be opt-in, preserve existing non-batching behavior, keep ordering/supervision deterministic, and remain location-transparent  
**Scale/Scope**: Single-node batching behavior for runtime mailbox execution; no distributed transport or persistence changes in this feature

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
specs/010-mailbox-batching/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── mailbox-batching-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── mailbox.go
│   ├── runtime.go
│   ├── actor_ref.go
│   ├── events.go
│   ├── outcome.go
│   └── types.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: Extend existing mailbox/runtime primitives under `src/actor` to support opt-in batch retrieval and outcomes, with behavior validated through contract/integration/unit suites.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/research.md). All technical-context unknowns are resolved with no open clarifications.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/contracts/mailbox-batching-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/contracts/mailbox-batching-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/quickstart.md)

## Post-Design Constitution Check

- [x] Batching design remains Go-native and in-memory without schema-heavy dependencies.
- [x] Batch failure outcomes are explicit and routed through existing supervision behavior.
- [x] Test-first strategy is captured for batch retrieval, bulk I/O optimization, and failure/recovery paths.
- [x] Public actor addressing remains location-transparent and unchanged for callers.
- [x] Batch retrieval bounds and fairness constraints keep hot-path complexity manageable and measurable.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
