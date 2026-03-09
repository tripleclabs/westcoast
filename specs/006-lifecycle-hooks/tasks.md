# Tasks: Lifecycle Management Hooks

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/006-lifecycle-hooks/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare lifecycle-hook test and runtime scaffolding.

- [X] T001 Create lifecycle integration test helper scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_hooks_test_helpers_test.go
- [X] T002 Create lifecycle contract test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/contract/lifecycle_hooks_contract_test.go
- [X] T003 [P] Create unit test scaffold for hook lifecycle invariants in /Volumes/Store1/src/3clabs/westcoast/tests/unit/lifecycle_hooks_unit_test.go
- [X] T004 [P] Confirm actor reference hook entrypoints in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T005 Confirm runtime lifecycle-hook touchpoints in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core lifecycle models and runtime hooks required before user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define lifecycle hook option and phase/result types in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Define lifecycle hook telemetry event types and fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T008 Implement actor-instance lifecycle hook state wiring in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T009 [P] Add lifecycle hook outcome recording primitives in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T010 [P] Add lifecycle hook metric interface methods in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T011 Add lifecycle metrics implementations in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go
- [X] T012 Add foundational contract tests for lifecycle outcome vocabulary in /Volumes/Store1/src/3clabs/westcoast/tests/contract/lifecycle_hooks_contract_test.go
- [X] T013 Add shared helper assertions for lifecycle start/stop outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_hooks_test_helpers_test.go

**Checkpoint**: Foundation ready - user stories can proceed in priority order

---

## Phase 3: User Story 1 - Initialize External Resources (Priority: P1) 🎯 MVP

**Goal**: Ensure Start hook runs before first message and startup failure blocks processing.

**Independent Test**: Start an actor with Start hook and verify first message sees initialized state; fail Start hook and verify no user messages are processed.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T014 [P] [US1] Add integration test for Start hook before first message in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_start_hook_before_first_message_test.go
- [X] T015 [P] [US1] Add integration test for Start hook failure blocking message processing in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_start_hook_failure_blocks_processing_test.go
- [X] T016 [US1] Add contract test for Start hook execution guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/contract/lifecycle_hooks_contract_test.go

### Implementation for User Story 1

- [X] T017 [P] [US1] Implement Start hook configuration and lifecycle wiring in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T018 [P] [US1] Add Start hook option exposure for actor construction in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T019 [US1] Enforce startup gate before user message processing in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T020 [US1] Emit start_success/start_failed lifecycle events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T021 [US1] Record Start hook outcomes for diagnostics in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Release Resources on Graceful Stop (Priority: P2)

**Goal**: Ensure Stop hook runs during graceful shutdown and actor reaches terminal stopped state even on hook failure.

**Independent Test**: Gracefully stop actors with Stop hooks and verify cleanup runs before final stop; force Stop hook failure and verify deterministic stopped outcome.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T022 [P] [US2] Add integration test for Stop hook on graceful shutdown in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_stop_hook_graceful_shutdown_test.go
- [X] T023 [P] [US2] Add integration test for Stop hook failure still reaching stopped state in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_stop_hook_failure_still_stops_test.go
- [X] T024 [US2] Add contract test for Stop hook shutdown guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/contract/lifecycle_hooks_contract_test.go

### Implementation for User Story 2

- [X] T025 [P] [US2] Implement Stop hook invocation in graceful shutdown path in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T026 [P] [US2] Add Stop hook option exposure for actor construction in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T027 [US2] Preserve terminal stopped transition on Stop hook failures in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T028 [US2] Emit stop_success/stop_failed lifecycle events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T029 [US2] Record Stop hook outcomes for diagnostics in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Observable Lifecycle Guarantees (Priority: P3)

**Goal**: Make Start/Stop hook outcomes consistently observable across normal and restart paths.

**Independent Test**: Run success/failure startup and shutdown flows (including supervised restart) and verify deterministic lifecycle outcome fields are emitted.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T030 [P] [US3] Add integration test for Start hook re-execution on supervision restart in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_start_hook_runs_on_restart_test.go
- [X] T031 [P] [US3] Add integration test for lifecycle outcome observability in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_outcomes_observable_test.go
- [X] T032 [US3] Add contract test for lifecycle outcome field requirements in /Volumes/Store1/src/3clabs/westcoast/tests/contract/lifecycle_hooks_contract_test.go

### Implementation for User Story 3

- [X] T033 [P] [US3] Integrate lifecycle hooks without changing supervision decision behavior in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T034 [P] [US3] Add lifecycle event payload fields for observability contract in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T035 [US3] Add lifecycle metrics observations for start/stop success/failure in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go
- [X] T036 [US3] Add startup-state message safety checks for queued delivery in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go

**Checkpoint**: All user stories are independently functional with lifecycle observability guarantees

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression, docs alignment, and concurrency validation.

- [X] T037 [P] Update lifecycle hook contract wording and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/006-lifecycle-hooks/contracts/lifecycle-management-hooks-contract.md
- [X] T038 [P] Update quickstart verification commands for lifecycle scenarios in /Volumes/Store1/src/3clabs/westcoast/specs/006-lifecycle-hooks/quickstart.md
- [X] T039 Run full verification suite (unit/integration/contract) from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression test for concurrent start-stop interleavings in /Volumes/Store1/src/3clabs/westcoast/tests/integration/lifecycle_hooks_concurrency_regression_test.go
- [X] T041 Confirm location-transparent lifecycle contract consistency in /Volumes/Store1/src/3clabs/westcoast/specs/006-lifecycle-hooks/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 startup hook baseline and actor stop integration
- **User Story 3 (P3)**: Depends on US1+US2 lifecycle hook execution paths and event surface

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Runtime lifecycle semantics before event/metrics wiring
- Contract and integration verification before story completion

### Parallel Opportunities

- Setup: T003 and T004 can run in parallel
- Foundational: T009 and T010 can run in parallel
- US1: T014 and T015 can run in parallel; T017 and T018 can run in parallel
- US2: T022 and T023 can run in parallel; T025 and T026 can run in parallel
- US3: T030 and T031 can run in parallel; T033 and T034 can run in parallel
- Polish: T037, T038, and T040 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "T014 [US1] Start hook before first message test in tests/integration/lifecycle_start_hook_before_first_message_test.go"
Task: "T015 [US1] Start hook failure blocking test in tests/integration/lifecycle_start_hook_failure_blocks_processing_test.go"
Task: "T017 [US1] Start hook runtime wiring in src/actor/runtime.go"
Task: "T018 [US1] Start hook actor options in src/actor/actor_ref.go"
```

## Parallel Example: User Story 2

```bash
Task: "T022 [US2] graceful shutdown Stop hook integration test in tests/integration/lifecycle_stop_hook_graceful_shutdown_test.go"
Task: "T023 [US2] Stop hook failure still stops test in tests/integration/lifecycle_stop_hook_failure_still_stops_test.go"
Task: "T025 [US2] Stop hook runtime stop path in src/actor/runtime.go"
Task: "T026 [US2] Stop hook actor options in src/actor/actor_ref.go"
```

## Parallel Example: User Story 3

```bash
Task: "T030 [US3] Start hook restart execution test in tests/integration/lifecycle_start_hook_runs_on_restart_test.go"
Task: "T031 [US3] lifecycle observability integration test in tests/integration/lifecycle_outcomes_observable_test.go"
Task: "T033 [US3] supervision-compatible lifecycle runtime integration in src/actor/runtime.go"
Task: "T034 [US3] lifecycle event payload contract fields in src/actor/events.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (startup initialization guarantees)
2. Deliver US2 (graceful shutdown cleanup guarantees)
3. Deliver US3 (observability and restart-path guarantees)
4. Execute polish and full regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split by story once foundation is complete
3. Recombine for final regression/docs/consistency completion
