# Implementation Plan: Request-Response Ask Semantics

**Branch**: `008-ask-reply-semantics` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/spec.md)
**Input**: Feature specification from `/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/spec.md`

## Summary

Add Ask-style request-response semantics with timeout-bounded waiting, implicit ReplyTo routing context, and asynchronous reply delegation so callers gain deterministic two-way messaging without blocking actor event loops.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `time`)  
**Storage**: In-memory Ask wait tracking and response-correlation state only (no persistence)  
**Testing**: `go test` across unit, integration, and contract suites with failing tests authored first  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Preserve actor loop responsiveness under asynchronous reply flows; avoid unbounded waiter growth; keep Ask bookkeeping overhead minimal relative to existing send path  
**Constraints**: Reply routing MUST remain PID-based and location-transparent; timed-out waits MUST release resources deterministically; late replies MUST not corrupt completed waits  
**Scale/Scope**: Feature includes Ask request lifecycle, ReplyTo context propagation, and deferred reply support in single-node runtime; distributed transport implementation is out of scope

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
specs/008-ask-reply-semantics/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── ask-reply-contract.md
└── tasks.md
```

### Source Code (repository root)

```text
src/
├── actor/
│   ├── runtime.go
│   ├── actor_ref.go
│   ├── pid.go
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

**Structure Decision**: Extend existing actor runtime messaging surfaces in `src/actor` to introduce Ask wait lifecycle and ReplyTo correlation while validating behavior in contract/integration/unit suites.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/research.md). All technical context items are resolved with no open clarifications.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/contracts/ask-reply-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/contracts/ask-reply-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/quickstart.md)

## Post-Design Constitution Check

- [x] Ask semantics remain Go-native and in-memory, without schema-heavy runtime dependencies.
- [x] Ask/reply failure cases are explicit, bounded, and compatible with supervisor-driven fault handling.
- [x] Test-first flow is captured for timeout, correlation, and async delegation behavior.
- [x] Reply routing remains PID-based and location-transparent for future multi-node transport.
- [x] Wait-state lifecycle and cleanup rules keep complexity and overhead predictable.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
