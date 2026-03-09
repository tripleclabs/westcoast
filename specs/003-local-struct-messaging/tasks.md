# Tasks: Type-Agnostic Local Messaging

**Input**: Design documents from `/specs/003-local-struct-messaging/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare local messaging scaffolding, benchmark entrypoints, and test layout

- [X] T001 Create local routing scaffold file in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T002 Create contract test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/contract/local_messaging_contract_test.go
- [X] T003 [P] Create benchmark scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/local_messaging_performance_test.go
- [X] T004 [P] Add benchmark target for local messaging profile in /Volumes/Store1/src/3clabs/westcoast/Makefile
- [X] T005 Confirm Go test package layout for new local messaging test files in /Volumes/Store1/src/3clabs/westcoast/go.mod

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core message typing, routing, and rejection primitives required by all stories

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define local message metadata fields (`type_name`, `schema_version`) in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Define deterministic rejection outcomes (`rejected_unsupported_type`, `rejected_nil_payload`, `rejected_version_mismatch`) in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T008 Implement type routing registry with exact-match + fallback semantics in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T009 [P] Add local routing event constants and payload fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T010 Implement message metadata extraction pipeline for local mailbox deliveries in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T011 [P] Add metrics hooks for local routing and local-send latency in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T012 Add foundational contract tests for terminal outcome set in /Volumes/Store1/src/3clabs/westcoast/tests/contract/local_messaging_contract_test.go

**Checkpoint**: Foundation ready - user stories can proceed in priority order

---

## Phase 3: User Story 1 - Send Native Local Messages (Priority: P1) 🎯 MVP

**Goal**: Deliver native structured payloads locally without encode/decode transformations.

**Independent Test**: Send native local payloads between actors and verify original shape and type metadata are preserved end-to-end.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add unit tests for native payload pass-through invariants in /Volumes/Store1/src/3clabs/westcoast/tests/unit/native_local_payload_test.go
- [X] T014 [P] [US1] Add integration tests for local sender-to-receiver native struct delivery in /Volumes/Store1/src/3clabs/westcoast/tests/integration/native_local_delivery_test.go
- [X] T015 [US1] Add contract tests for local message metadata preservation in /Volumes/Store1/src/3clabs/westcoast/tests/contract/local_messaging_contract_test.go

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement native payload local send path in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T017 [P] [US1] Implement mailbox delivery flow preserving native payloads in /Volumes/Store1/src/3clabs/westcoast/src/actor/mailbox.go
- [X] T018 [US1] Implement runtime handler invocation with preserved local type metadata in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T019 [US1] Enforce no local serialization stage in intra-node path in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T020 [US1] Emit `message_routed_exact` and `message_processed` events for successful local delivery in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Enforce Zero-Serialization in Intra-Node Paths (Priority: P2)

**Goal**: Guarantee local-path messaging never performs encode/decode and returns deterministic rejections.

**Independent Test**: Run local messaging under varied payload classes and verify no serialization stage plus deterministic rejection outcomes.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T021 [P] [US2] Add integration tests for unsupported type rejection (`rejected_unsupported_type`) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/reject_unsupported_type_test.go
- [X] T022 [P] [US2] Add integration tests for nil payload rejection (`rejected_nil_payload`) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/reject_nil_payload_test.go
- [X] T023 [US2] Add contract tests for version mismatch rejection (`rejected_version_mismatch`) in /Volumes/Store1/src/3clabs/westcoast/tests/contract/local_messaging_contract_test.go

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement unsupported-type rejection path in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T025 [P] [US2] Implement nil payload guard and rejection outcome in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T026 [US2] Implement schema-version validation and mismatch rejection in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T027 [US2] Emit rejection events with canonical outcome labels in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T028 [US2] Persist deterministic local handling outcomes in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Route Logic by Message Type (Priority: P3)

**Goal**: Provide deterministic exact-type-first message routing with optional explicit fallback and measurable performance.

**Independent Test**: Deliver mixed typed payloads and verify exact-match routing precedence, fallback behavior, and performance gates.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T029 [P] [US3] Add unit tests for type-routing precedence and fallback resolution in /Volumes/Store1/src/3clabs/westcoast/tests/unit/type_routing_precedence_test.go
- [X] T030 [P] [US3] Add integration tests for mixed-type dispatch correctness in /Volumes/Store1/src/3clabs/westcoast/tests/integration/mixed_type_dispatch_test.go
- [X] T031 [US3] Add benchmark tests for local send performance gates in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/local_messaging_performance_test.go

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement exact type+version matching evaluator in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T033 [P] [US3] Implement optional explicit fallback handler route in /Volumes/Store1/src/3clabs/westcoast/src/actor/routing.go
- [X] T034 [US3] Instrument local send latency and throughput metrics in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go
- [X] T035 [US3] Implement benchmark harness asserting p95 <= 25 µs and >=1,000,000 msgs/sec in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/local_messaging_performance_test.go
- [X] T036 [US3] Add contract validation for routing determinism across type/version combinations in /Volumes/Store1/src/3clabs/westcoast/tests/contract/local_messaging_contract_test.go

**Checkpoint**: All user stories are independently functional and benchmark-validated

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression, docs alignment, and end-to-end validation

- [X] T037 [P] Update local messaging contract details and invariant wording in /Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/contracts/local-messaging-contract.md
- [X] T038 [P] Update quickstart benchmark and validation commands in /Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/quickstart.md
- [X] T039 Run full unit/integration/contract/benchmark verification from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression tests for ambiguous handler overlap and fallback edge cases in /Volumes/Store1/src/3clabs/westcoast/tests/integration/local_messaging_edge_cases_test.go
- [X] T041 Confirm terminology consistency (`type_name`, `schema_version`, rejection outcomes) in /Volumes/Store1/src/3clabs/westcoast/specs/003-local-struct-messaging/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all targeted user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 native local delivery baseline
- **User Story 3 (P3)**: Depends on US1+US2 routing correctness and rejection semantics

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Core routing/data primitives before runtime path integration
- Contract/integration verification before story completion

### Parallel Opportunities

- Setup: T003 and T004 can run in parallel
- Foundational: T009 and T011 can run in parallel with T006/T008/T010
- US1: T013 and T014 can run in parallel; T016 and T017 can run in parallel
- US2: T021 and T022 can run in parallel; T024 and T025 can run in parallel
- US3: T029 and T030 can run in parallel; T032 and T033 can run in parallel
- Polish: T037, T038, and T040 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "T013 [US1] native payload pass-through tests in tests/unit/native_local_payload_test.go"
Task: "T014 [US1] native local delivery integration tests in tests/integration/native_local_delivery_test.go"
Task: "T016 [US1] native local send path in src/actor/actor_ref.go"
Task: "T017 [US1] mailbox local payload flow in src/actor/mailbox.go"
```

## Parallel Example: User Story 2

```bash
Task: "T021 [US2] unsupported type rejection tests in tests/integration/reject_unsupported_type_test.go"
Task: "T022 [US2] nil payload rejection tests in tests/integration/reject_nil_payload_test.go"
Task: "T024 [US2] unsupported-type rejection path in src/actor/routing.go"
Task: "T025 [US2] nil payload guard path in src/actor/runtime.go"
```

## Parallel Example: User Story 3

```bash
Task: "T029 [US3] routing precedence unit tests in tests/unit/type_routing_precedence_test.go"
Task: "T030 [US3] mixed type dispatch integration tests in tests/integration/mixed_type_dispatch_test.go"
Task: "T032 [US3] exact type+version routing evaluator in src/actor/routing.go"
Task: "T033 [US3] explicit fallback handler route in src/actor/routing.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (native local message pass-through)
2. Deliver US2 (zero-serialization guarantees + deterministic rejections)
3. Deliver US3 (pattern-matching emulation + performance gate)
4. Execute polish and full regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split by user story with shared contract checkpoints
3. Recombine for phase 6 final verification and documentation alignment
