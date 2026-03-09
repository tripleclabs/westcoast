# Tasks: Supervisor Trees & Fault Tolerance

**Input**: Design documents from `/specs/004-supervisor-fault-tolerance/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare supervision test scaffolding and runtime extension points.

- [X] T001 Create supervision test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/integration/supervision_test_helpers.go
- [X] T002 Create supervision contract test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/contract/supervision_fault_tolerance_contract_test.go
- [X] T003 [P] Add unit test scaffold for restart lifecycle behavior in /Volumes/Store1/src/3clabs/westcoast/tests/unit/restart_lifecycle_test.go
- [X] T004 [P] Add benchmark placeholder for recovery-path overhead in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/supervision_recovery_benchmark_test.go
- [X] T005 Confirm runtime/supervisor package boundaries for feature changes in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core supervision/failure lifecycle primitives required before user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define explicit supervision failure classes and restart lifecycle result enums in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Define failure lifecycle event types and fields for supervision decisions in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T008 Implement deterministic supervision decision mapping (restart/stop/escalate) in /Volumes/Store1/src/3clabs/westcoast/src/actor/supervisor.go
- [X] T009 [P] Add restart counter and per-actor lifecycle metadata fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T010 [P] Extend outcome model for crash-triggering message terminal failure recording in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T011 Add foundational contract tests for supervision outcome set in /Volumes/Store1/src/3clabs/westcoast/tests/contract/supervision_fault_tolerance_contract_test.go
- [X] T012 Add shared integration assertions for actor-liveness after peer panic in /Volumes/Store1/src/3clabs/westcoast/tests/integration/supervision_test_helpers.go

**Checkpoint**: Foundation ready - user stories can proceed in priority order

---

## Phase 3: User Story 1 - Isolate Actor Crashes (Priority: P1) 🎯 MVP

**Goal**: Panic in one actor is intercepted and isolated without runtime-wide outage.

**Independent Test**: Trigger panic in one actor while another actor handles new messages; verify runtime remains responsive and failure telemetry is emitted.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add integration test for actor panic isolation and healthy peer continuity in /Volumes/Store1/src/3clabs/westcoast/tests/integration/actor_panic_isolation_test.go
- [X] T014 [P] [US1] Add integration test for structured panic failure event emission in /Volumes/Store1/src/3clabs/westcoast/tests/integration/panic_failure_event_test.go
- [X] T015 [US1] Add contract test for actor-local crash interception guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/contract/supervision_fault_tolerance_contract_test.go

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement panic interception boundary in message processing loop in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T017 [P] [US1] Record crash-triggering message terminal failure outcome in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T018 [US1] Emit actor_failed event with actor/message identity and failure class in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T019 [US1] Ensure non-crashing actors remain schedulable after peer panic in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T020 [US1] Add runtime metrics hook call for panic interception count in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Restart with Clean State (Priority: P2)

**Goal**: Restarted actors recover from crash with fresh initial state and deterministic policy behavior.

**Independent Test**: Mutate actor state, force panic, verify restarted actor state equals initial snapshot and limit behavior is deterministic.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T021 [P] [US2] Add integration test for clean initial-state reset after restart in /Volumes/Store1/src/3clabs/westcoast/tests/integration/clean_state_restart_test.go
- [X] T022 [P] [US2] Add integration test for deterministic restart-limit handling in /Volumes/Store1/src/3clabs/westcoast/tests/integration/restart_limit_determinism_test.go
- [X] T023 [US2] Add contract test for restart/stop/escalate decision determinism in /Volumes/Store1/src/3clabs/westcoast/tests/contract/supervision_fault_tolerance_contract_test.go

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement actor state reset to initial snapshot on restart decision in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T025 [P] [US2] Implement lifecycle transition updates failed->restarting->running in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T026 [US2] Implement post-limit supervision behavior branch in /Volumes/Store1/src/3clabs/westcoast/src/actor/supervisor.go
- [X] T027 [US2] Emit actor_restarted and terminal stop/escalate events with decision metadata in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T028 [US2] Persist restart_count and latest supervision decision in runtime actor metadata in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Preserve Mailbox During Restart (Priority: P3)

**Goal**: Unprocessed queued mailbox messages survive restart and keep FIFO ordering guarantees.

**Independent Test**: Queue a message sequence around a panic, restart actor, and verify queued-unprocessed messages are retained and processed in FIFO order.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T029 [P] [US3] Add integration test for queued-unprocessed mailbox preservation across restart in /Volumes/Store1/src/3clabs/westcoast/tests/integration/mailbox_preservation_restart_test.go
- [X] T030 [P] [US3] Add integration test for FIFO ordering of preserved mailbox messages post-restart in /Volumes/Store1/src/3clabs/westcoast/tests/integration/mailbox_fifo_after_restart_test.go
- [X] T031 [US3] Add contract test for mailbox preservation and ordering guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/contract/supervision_fault_tolerance_contract_test.go

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement restart path that preserves queued-unprocessed mailbox entries in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T033 [P] [US3] Implement restart-safe dequeue boundary so crash-triggering message is failed while later queued messages remain pending in /Volumes/Store1/src/3clabs/westcoast/src/actor/mailbox.go
- [X] T034 [US3] Enforce FIFO processing continuity for preserved queue entries after restart in /Volumes/Store1/src/3clabs/westcoast/src/actor/mailbox.go
- [X] T035 [US3] Emit supervision telemetry for mailbox-preserved replay lifecycle in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T036 [US3] Add runtime metric observations for preserved queue depth and post-restart drain behavior in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go

**Checkpoint**: All user stories are independently functional and recovery semantics are verified

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression, contract/doc alignment, and recovery-path validation.

- [X] T037 [P] Update supervision contract wording and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/contracts/supervision-fault-tolerance-contract.md
- [X] T038 [P] Update quickstart validation commands and expected outcomes in /Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/quickstart.md
- [X] T039 Run full verification suite (unit/integration/contract) from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression tests for restart-storm and stop-policy mailbox handling edge cases in /Volumes/Store1/src/3clabs/westcoast/tests/integration/supervision_edge_cases_test.go
- [X] T041 Confirm location-transparent caller contract continuity during restart in /Volumes/Store1/src/3clabs/westcoast/specs/004-supervisor-fault-tolerance/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 crash interception baseline
- **User Story 3 (P3)**: Depends on US1+US2 recovery lifecycle guarantees

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Runtime failure primitives before lifecycle/event integration
- Contract and integration verification before story completion

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
Task: "T013 [US1] panic isolation integration test in tests/integration/actor_panic_isolation_test.go"
Task: "T014 [US1] panic failure event integration test in tests/integration/panic_failure_event_test.go"
Task: "T016 [US1] panic interception boundary in src/actor/runtime.go"
Task: "T017 [US1] crash-triggering message terminal outcome in src/actor/runtime.go"
```

## Parallel Example: User Story 2

```bash
Task: "T021 [US2] clean state restart integration test in tests/integration/clean_state_restart_test.go"
Task: "T022 [US2] restart-limit determinism integration test in tests/integration/restart_limit_determinism_test.go"
Task: "T024 [US2] state reset on restart in src/actor/runtime.go"
Task: "T025 [US2] lifecycle transitions in src/actor/runtime.go"
```

## Parallel Example: User Story 3

```bash
Task: "T029 [US3] mailbox preservation integration test in tests/integration/mailbox_preservation_restart_test.go"
Task: "T030 [US3] FIFO after restart integration test in tests/integration/mailbox_fifo_after_restart_test.go"
Task: "T032 [US3] preserve queued-unprocessed mailbox entries in src/actor/runtime.go"
Task: "T033 [US3] restart-safe dequeue boundary in src/actor/mailbox.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (panic isolation and actor-local crash interception)
2. Deliver US2 (clean restarts and deterministic supervision limits)
3. Deliver US3 (mailbox preservation and FIFO continuity through restart)
4. Execute polish and final regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split per story with shared runtime checkpoints
3. Recombine for Phase 6 contract/docs/regression completion
