# Tasks: Local Actor Registry & Discovery

**Input**: Design documents from `/specs/005-actor-registry-discovery/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare registry/discovery test and runtime scaffolding.

- [X] T001 Create named registry integration test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/integration/registry_discovery_test_helpers_test.go
- [X] T002 Create registry contract test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/contract/actor_registry_discovery_contract_test.go
- [X] T003 [P] Add unit test scaffold for registry race-safety behavior in /Volumes/Store1/src/3clabs/westcoast/tests/unit/registry_concurrency_test.go
- [X] T004 [P] Add benchmark scaffold for lookup latency under contention in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/registry_lookup_benchmark_test.go
- [X] T005 Confirm runtime-to-registry integration touchpoints in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core registry models and runtime hooks required before user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define registry operation/result enums and error outcomes in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Define registry telemetry event types and fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T008 Implement core name reservation and uniqueness guard primitives in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T009 [P] Add runtime references to registry directory and lifecycle bindings in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T010 [P] Add registry operation metric hook interfaces in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T011 Add foundational contract tests for registry operation outcome set in /Volumes/Store1/src/3clabs/westcoast/tests/contract/actor_registry_discovery_contract_test.go
- [X] T012 Add shared helper assertions for register/lookup/unregister outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/integration/registry_discovery_test_helpers_test.go

**Checkpoint**: Foundation ready - user stories can proceed in priority order

---

## Phase 3: User Story 1 - Register Actors by Name (Priority: P1) 🎯 MVP

**Goal**: Allow actors to register under unique human-readable names.

**Independent Test**: Register a running actor with an unused name and verify success; attempt duplicate registration and verify deterministic reject.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add integration test for successful unique name registration in /Volumes/Store1/src/3clabs/westcoast/tests/integration/named_registration_unique_test.go
- [X] T014 [P] [US1] Add integration test for duplicate name rejection determinism in /Volumes/Store1/src/3clabs/westcoast/tests/integration/named_registration_duplicate_test.go
- [X] T015 [US1] Add contract test for registration validation and uniqueness in /Volumes/Store1/src/3clabs/westcoast/tests/contract/actor_registry_discovery_contract_test.go

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement named registration API and validation in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T017 [P] [US1] Implement deterministic duplicate rejection path in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T018 [US1] Add runtime-level registration entry point for actors in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T019 [US1] Emit register_success/register_rejected_duplicate telemetry events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T020 [US1] Record registration operation outcomes for diagnostics in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Resolve Name to PID Dynamically (Priority: P2)

**Goal**: Provide deterministic name-to-PID discovery lookups with explicit not-found outcomes.

**Independent Test**: Register multiple names and verify each lookup returns the correct PID while unknown names return not-found.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T021 [P] [US2] Add integration test for lookup hit returning correct PID in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lookup_by_name_returns_pid_test.go
- [X] T022 [P] [US2] Add integration test for lookup miss deterministic not-found in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lookup_by_name_not_found_test.go
- [X] T023 [US2] Add contract test for lookup hit/miss semantics in /Volumes/Store1/src/3clabs/westcoast/tests/contract/actor_registry_discovery_contract_test.go

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement lookup-by-name read path returning PID result in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T025 [P] [US2] Implement deterministic lookup_not_found outcome path in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T026 [US2] Add runtime helper for discovery lookup integration in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T027 [US2] Emit lookup_hit/lookup_not_found telemetry events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T028 [US2] Add metrics observation for lookup latency and hit/miss counts in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Keep Registry in Sync with Actor Lifecycle (Priority: P3)

**Goal**: Automatically remove named mappings when actors terminate permanently.

**Independent Test**: Register actor name, terminate actor gracefully or permanently, verify future lookup returns not-found and no stale mapping remains.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T029 [P] [US3] Add integration test for graceful-stop lifecycle unregister in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_sync_graceful_stop_test.go
- [X] T030 [P] [US3] Add integration test for permanent supervision termination unregister in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_sync_supervision_terminal_test.go
- [X] T031 [US3] Add contract test for lifecycle-triggered unregister semantics in /Volumes/Store1/src/3clabs/westcoast/tests/contract/actor_registry_discovery_contract_test.go

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement lifecycle binding from actor terminal stop to registry cleanup in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T033 [P] [US3] Implement unregister_lifecycle_terminal path in registry store in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T034 [US3] Preserve registration across temporary restart while removing on terminal outcomes in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T035 [US3] Emit lifecycle-driven unregister telemetry event in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T036 [US3] Add stale-entry prevention checks for post-terminal lookups in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go

**Checkpoint**: All user stories are independently functional and lifecycle-sync semantics are verified

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression, docs alignment, and concurrency validation.

- [X] T037 [P] Update registry/discovery contract wording and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/contracts/actor-registry-discovery-contract.md
- [X] T038 [P] Update quickstart verification commands for registry scenarios in /Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/quickstart.md
- [X] T039 Run full verification suite (unit/integration/contract/benchmark) from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression tests for concurrent register/lookup/remove race determinism in /Volumes/Store1/src/3clabs/westcoast/tests/integration/registry_concurrent_determinism_test.go
- [X] T041 Confirm location-transparent discovery contract consistency in /Volumes/Store1/src/3clabs/westcoast/specs/005-actor-registry-discovery/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 registration baseline
- **User Story 3 (P3)**: Depends on US1+US2 registry ownership and lookup correctness

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Registry models/ownership checks before runtime wiring
- Contract/integration verification before story completion

### Parallel Opportunities

- Setup: T003 and T004 can run in parallel
- Foundational: T009 and T010 can run in parallel with T006/T008
- US1: T013 and T014 can run in parallel; T016 and T017 can run in parallel
- US2: T021 and T022 can run in parallel; T024 and T025 can run in parallel
- US3: T029 and T030 can run in parallel; T032 and T033 can run in parallel
- Polish: T037, T038, and T040 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "T013 [US1] unique name registration test in tests/integration/named_registration_unique_test.go"
Task: "T014 [US1] duplicate registration determinism test in tests/integration/named_registration_duplicate_test.go"
Task: "T016 [US1] named registration API in src/actor/registry.go"
Task: "T017 [US1] duplicate rejection path in src/actor/registry.go"
```

## Parallel Example: User Story 2

```bash
Task: "T021 [US2] lookup hit integration test in tests/integration/lookup_by_name_returns_pid_test.go"
Task: "T022 [US2] lookup miss integration test in tests/integration/lookup_by_name_not_found_test.go"
Task: "T024 [US2] lookup-by-name read path in src/actor/registry.go"
Task: "T025 [US2] deterministic not-found path in src/actor/registry.go"
```

## Parallel Example: User Story 3

```bash
Task: "T029 [US3] graceful-stop unregister integration test in tests/integration/lifecycle_sync_graceful_stop_test.go"
Task: "T030 [US3] terminal supervision unregister integration test in tests/integration/lifecycle_sync_supervision_terminal_test.go"
Task: "T032 [US3] terminal lifecycle binding to cleanup in src/actor/runtime.go"
Task: "T033 [US3] unregister_lifecycle_terminal path in src/actor/registry.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (named registration + duplicate rejection)
2. Deliver US2 (dynamic lookup hit/miss semantics)
3. Deliver US3 (lifecycle-synced automatic unregister)
4. Execute polish and full regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split per story with shared registry contract checkpoints
3. Recombine for Phase 6 regression/docs/consistency completion
