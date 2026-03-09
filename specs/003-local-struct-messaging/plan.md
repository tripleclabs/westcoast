# Implementation Plan: Type-Agnostic Local Messaging

**Branch**: `003-local-struct-messaging` | **Date**: 2026-03-08 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/spec.md)
**Input**: Feature specification from `/specs/003-local-struct-messaging/spec.md`

## Summary

Implement a local messaging path that passes native structured payloads directly between actors with
zero local serialization overhead. The design enforces deterministic type-based routing,
well-defined rejection outcomes for invalid payload classes, and measurable local performance gates.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `reflect`, `time`)  
**Storage**: In-memory mailbox/message structures only (no persistence in this feature)  
**Testing**: `go test` unit/integration/benchmark suites with type-routing and zero-serialization checks  
**Target Platform**: Linux/macOS single-node runtime
**Project Type**: Runtime library  
**Performance Goals**: p95 local send latency <= 25 µs and throughput >= 1,000,000 msgs/sec on baseline profile  
**Constraints**: No local encode/decode in intra-node path, exact-type-first routing, explicit fallback,
`rejected_unsupported_type`, `rejected_nil_payload`, `rejected_version_mismatch` outcomes,
location-transparent caller contracts preserved  
**Scale/Scope**: Single-node local message transport path and type-routing semantics only;
cross-node serialization remains out of scope

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
specs/003-local-struct-messaging/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── local-messaging-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── mailbox.go
│   ├── types.go
│   ├── actor_ref.go
│   └── routing.go
└── internal/
    └── metrics/

tests/
├── contract/
├── integration/
├── unit/
└── benchmark/
```

**Structure Decision**: Extend existing actor runtime primitives with explicit local message routing
and rejection outcome handling while preserving current package layout.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/research.md). All unknowns are resolved.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/contracts/local-messaging-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/contracts/local-messaging-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/quickstart.md)

## Post-Design Constitution Check

- [x] Go-native messaging preserved; no external serializer introduced in local path.
- [x] Failure and rejection outcomes are explicit and compatible with supervision behavior.
- [x] TDD-first flow captured in quickstart and contract validation path.
- [x] Existing location-transparent contracts remain unchanged for callers.
- [x] Performance gates are measurable and benchmark-backed.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
