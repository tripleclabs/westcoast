# Implementation Plan: Native Pub/Sub Event Bus

**Branch**: `011-native-pubsub-broker` | **Date**: 2026-03-09 | **Spec**: [/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/spec.md](/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/spec.md)
**Input**: Feature specification from `/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/spec.md`

## Summary

Add a built-in, zero-dependency broker actor with Trie-based topic subscriptions, wildcard routing, Ask-driven dynamic subscription management, and asynchronous publish fan-out to matching subscriber PIDs while preserving location-transparent actor contracts.

## Technical Context

**Language/Version**: Go 1.24+  
**Primary Dependencies**: Go standard library (`context`, `sync`, `sync/atomic`, `strings`, `time`)  
**Storage**: In-memory broker subscription Trie, subscriber index maps, and broker outcomes only (no persistence)  
**Testing**: `go test` unit/integration/contract suites with failing tests authored before implementation  
**Target Platform**: Linux/macOS single-node runtime  
**Project Type**: Actor runtime library  
**Performance Goals**: Low-latency subscription matching for local publish paths, asynchronous fan-out without publisher-side blocking, predictable subscription update behavior under concurrent changes  
**Constraints**: Built-in broker only (no external MQ dependency), Trie-based segment routing with `+` and `#` wildcard semantics, Ask-based subscribe/unsubscribe commands, location-transparent PID delivery contracts  
**Scale/Scope**: Single-node broker behavior for intra-node Pub/Sub; no inter-node transport in this feature

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
specs/011-native-pubsub-broker/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── native-pubsub-broker-contract.md
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

**Structure Decision**: Extend existing runtime/actor primitives in `src/actor` with broker lifecycle, subscription Trie evaluation, Ask command handling, and publish fan-out; verify behavior through contract/integration/unit suites.

## Phase 0: Research Outcomes

See [/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/research.md](/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/research.md). All technical context questions are resolved with no open clarifications.

## Phase 1: Design Outputs

- Data model: [/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/data-model.md](/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/data-model.md)
- Contracts: [/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/contracts/native-pubsub-broker-contract.md](/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/contracts/native-pubsub-broker-contract.md)
- Quickstart: [/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/quickstart.md](/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/quickstart.md)

## Post-Design Constitution Check

- [x] Broker design remains Go-native and in-memory with no external queue dependencies.
- [x] Broker and delivery-target failures map to explicit, supervision-compatible outcomes.
- [x] Test-first strategy is captured for exact/wildcard routing, dynamic subscriptions, and fan-out behavior.
- [x] Caller/subscriber contracts remain location-transparent via logical actor identity and PIDs.
- [x] Trie matching and fan-out design keep hot-path behavior bounded and measurable.

Gate Status (Post-Design): PASS

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| None | N/A | N/A |
