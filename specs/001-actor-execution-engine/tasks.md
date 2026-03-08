# Tasks: Actor Execution Engine

**Input**: Design documents from `/specs/001-actor-execution-engine/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and baseline tooling

- [X] T001 Create actor runtime directories in /Volumes/Store1/src/3clabs/westcoast/src/actor and /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics
- [X] T002 Create test directories in /Volumes/Store1/src/3clabs/westcoast/tests/unit, /Volumes/Store1/src/3clabs/westcoast/tests/integration, and /Volumes/Store1/src/3clabs/westcoast/tests/benchmark
- [X] T003 Initialize Go module metadata and baseline build settings in /Volumes/Store1/src/3clabs/westcoast/go.mod
- [X] T004 [P] Create runtime package skeleton and public type declarations in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T005 [P] Configure formatting/lint entrypoint in /Volumes/Store1/src/3clabs/westcoast/Makefile

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define core runtime entities and lifecycle enums in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Implement logical actor ID registry and uniqueness checks in /Volumes/Store1/src/3clabs/westcoast/src/actor/registry.go
- [X] T008 Implement runtime event envelope and emission hooks in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T009 [P] Implement metrics hook interfaces for mailbox depth and processing latency in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T010 Implement supervision policy interfaces (restart/stop/escalate) in /Volumes/Store1/src/3clabs/westcoast/src/actor/supervisor.go
- [X] T011 Implement actor reference abstraction using logical IDs in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T012 Add foundational contract conformance tests for actor ID addressing in /Volumes/Store1/src/3clabs/westcoast/tests/integration/contract_addressing_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Run Isolated Actors (Priority: P1) 🎯 MVP

**Goal**: Actors run with private state that can only mutate through their own message handling.

**Independent Test**: Create two actors, send mutation messages to one actor, verify cross-actor state mutation is impossible and direct external mutation is rejected.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add unit tests for state isolation invariants in /Volumes/Store1/src/3clabs/westcoast/tests/unit/actor_state_isolation_test.go
- [X] T014 [P] [US1] Add integration tests for rejected direct state mutation attempts in /Volumes/Store1/src/3clabs/westcoast/tests/integration/direct_state_mutation_test.go
- [X] T015 [US1] Add contract tests for create/query actor lifecycle operations in /Volumes/Store1/src/3clabs/westcoast/tests/integration/contract_actor_lifecycle_test.go

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement actor lifecycle create/start/stop transitions in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T017 [P] [US1] Implement actor-owned state container and guarded mutation API in /Volumes/Store1/src/3clabs/westcoast/src/actor/state.go
- [X] T018 [US1] Implement create and status query public operations in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T019 [US1] Implement state revision updates only within message execution context in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T020 [US1] Emit lifecycle and mutation outcome events for isolated actors in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Process Messages Asynchronously (Priority: P2)

**Goal**: Producers enqueue messages without blocking while each actor processes mailbox messages sequentially in acceptance order.

**Independent Test**: Rapidly enqueue messages and verify enqueue responsiveness, FIFO order, and explicit outcomes for full/stopped/not-found cases.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T021 [P] [US2] Add unit tests for non-blocking enqueue behavior in /Volumes/Store1/src/3clabs/westcoast/tests/unit/mailbox_enqueue_nonblocking_test.go
- [X] T022 [P] [US2] Add unit tests for per-actor FIFO processing order in /Volumes/Store1/src/3clabs/westcoast/tests/unit/mailbox_ordering_test.go
- [X] T023 [US2] Add integration tests for send-message contract outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/integration/contract_send_message_test.go

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement bounded mailbox queue with explicit reject-on-full outcomes in /Volumes/Store1/src/3clabs/westcoast/src/actor/mailbox.go
- [X] T025 [P] [US2] Implement asynchronous send operation returning acceptance/rejection codes in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T026 [US2] Implement per-actor sequential dispatch loop in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T027 [US2] Implement processing outcome records for success/failed/rejected states in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T028 [US2] Emit message processed/rejected events with result metadata in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go

**Checkpoint**: User Stories 1 and 2 are independently functional

---

## Phase 5: User Story 3 - Support Massive Concurrency on One Node (Priority: P3)

**Goal**: Runtime scales to very high actor counts while maintaining stability and service continuity.

**Independent Test**: Start actors in increasing batches up to target scale and verify actors remain available and process messages under representative load.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T029 [P] [US3] Add benchmark for high-density actor creation and steady-state operation in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/benchmark_million_actors_test.go
- [X] T030 [P] [US3] Add integration test for actor failure isolation and unaffected peer continuity in /Volumes/Store1/src/3clabs/westcoast/tests/integration/actor_failure_isolation_test.go
- [X] T031 [US3] Add integration test for restart policy behavior and recovery timing in /Volumes/Store1/src/3clabs/westcoast/tests/integration/actor_restart_recovery_test.go

### Implementation for User Story 3

- [X] T032 [P] [US3] Optimize actor runtime hot paths to minimize allocations in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T033 [P] [US3] Implement worker orchestration for high-concurrency actor scheduling in /Volumes/Store1/src/3clabs/westcoast/src/actor/scheduler.go
- [X] T034 [US3] Implement failure isolation and supervision policy application in /Volumes/Store1/src/3clabs/westcoast/src/actor/supervisor.go
- [X] T035 [US3] Instrument enqueue/processing/restart metrics for scale validation in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go
- [X] T036 [US3] Implement stop/query behavior guarantees for overloaded and restarting actors in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go

**Checkpoint**: All user stories are independently functional

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T037 [P] Update contract examples and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/contracts/actor-runtime-contract.md
- [X] T038 [P] Update quickstart verification steps with final commands in /Volumes/Store1/src/3clabs/westcoast/specs/001-actor-execution-engine/quickstart.md
- [X] T039 Run full test suite and benchmark gates from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression tests for edge cases (full mailbox, stopped actor, duplicate messages) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/edge_cases_test.go
- [X] T041 Confirm location-transparent contract guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/integration/contract_addressing_test.go

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all targeted user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Starts after Foundational; relies on actor lifecycle and IDs from US1
- **User Story 3 (P3)**: Starts after Foundational; builds on mailbox/runtime behavior from US1+US2

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the MVP release slice

### Within Each User Story

- Write tests and confirm failure before implementation
- Implement core model/runtime primitives before higher-level operations
- Complete story verification before moving to next priority

### Parallel Opportunities

- Setup: T004 and T005 can run in parallel
- Foundational: T009 can run in parallel with T007/T008/T010
- US1: T013 and T014 can run in parallel; T016 and T017 can run in parallel
- US2: T021 and T022 can run in parallel; T024 and T025 can run in parallel
- US3: T029 and T030 can run in parallel; T032 and T033 can run in parallel
- Polish: T037, T038, and T040 can run in parallel

---

## Parallel Example: User Story 1

```bash
Task: "T013 [US1] state isolation tests in tests/unit/actor_state_isolation_test.go"
Task: "T014 [US1] direct mutation rejection tests in tests/integration/direct_state_mutation_test.go"
Task: "T016 [US1] lifecycle transitions in src/actor/runtime.go"
Task: "T017 [US1] guarded state container in src/actor/state.go"
```

## Parallel Example: User Story 2

```bash
Task: "T021 [US2] enqueue non-blocking tests in tests/unit/mailbox_enqueue_nonblocking_test.go"
Task: "T022 [US2] FIFO ordering tests in tests/unit/mailbox_ordering_test.go"
Task: "T024 [US2] bounded mailbox in src/actor/mailbox.go"
Task: "T025 [US2] async send outcomes in src/actor/actor_ref.go"
```

## Parallel Example: User Story 3

```bash
Task: "T029 [US3] scale benchmark in tests/benchmark/benchmark_million_actors_test.go"
Task: "T030 [US3] failure isolation integration test in tests/integration/actor_failure_isolation_test.go"
Task: "T032 [US3] runtime hot-path optimization in src/actor/runtime.go"
Task: "T033 [US3] high-concurrency scheduler in src/actor/scheduler.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before expanding scope

### Incremental Delivery

1. Deliver US1 (state isolation + lifecycle)
2. Deliver US2 (asynchronous mailbox processing)
3. Deliver US3 (high-density concurrency + recovery performance)
4. Execute Polish phase and final regression gates

### Parallel Team Strategy

1. Team completes Setup + Foundational together
2. Then split by story priority with shared contract checks
3. Reintegrate at Phase 6 for full-system validation

