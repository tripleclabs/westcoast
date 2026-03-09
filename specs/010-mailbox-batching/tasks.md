# Tasks: Smart Mailboxes and Message Batching

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/010-mailbox-batching/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/mailbox-batching-contract.md, quickstart.md

**Tests**: Test tasks are REQUIRED for this feature. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story for independent implementation and validation.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare batching test scaffolding and shared observability placeholders.

- [X] T001 Create shared batching integration helpers in tests/integration/batching_test_helpers_test.go
- [X] T002 Create mailbox batching contract test scaffold in tests/contract/mailbox_batching_contract_test.go
- [X] T003 [P] Add batch outcome metric hook placeholders in src/internal/metrics/hooks.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build runtime primitives required by all batching user stories.

**⚠️ CRITICAL**: Complete this phase before user story work.

- [X] T004 Define batching domain types and outcomes in src/actor/types.go
- [X] T005 Add batch lifecycle event fields/types in src/actor/events.go
- [X] T006 Implement batch outcome storage/query primitives in src/actor/outcome.go
- [X] T007 Add mailbox bounded bulk-dequeue primitives in src/actor/mailbox.go
- [X] T008 Add runtime actor batching configuration registry in src/actor/runtime.go
- [X] T009 [P] Add actor reference surfaces for batching configuration in src/actor/actor_ref.go

**Checkpoint**: Foundation complete; user stories can proceed.

---

## Phase 3: User Story 1 - Process Batched Workloads (Priority: P1) 🎯 MVP

**Goal**: Allow opt-in actors to consume multiple queued messages per execution cycle with bounded batch size.

**Independent Test**: Configure batching-enabled actor, enqueue burst messages, and verify cycles process bounded multi-message batches while preserving ordering guarantees.

### Tests for User Story 1 (REQUIRED)

- [X] T010 [P] [US1] Add integration test for multi-message batch cycle retrieval in tests/integration/batch_receive_consumes_multiple_messages_per_cycle_test.go
- [X] T011 [P] [US1] Add integration test for configured batch-size bound enforcement in tests/integration/batch_receive_respects_configured_batch_limit_test.go
- [X] T012 [P] [US1] Add integration test for non-batching actor compatibility in tests/integration/non_batched_actor_behavior_unchanged_test.go
- [X] T013 [P] [US1] Add unit test for mailbox bulk-dequeue ordering and bounds in tests/unit/mailbox_bulk_dequeue_ordering_test.go
- [X] T014 [P] [US1] Add contract assertions for batch retrieval semantics in tests/contract/mailbox_batching_contract_test.go

### Implementation for User Story 1

- [X] T015 [US1] Implement runtime batch execution cycle (single actor cycle consumes bounded batch) in src/actor/runtime.go
- [X] T016 [US1] Integrate batch dequeue with mailbox ordering guarantees in src/actor/mailbox.go
- [X] T017 [US1] Emit batch success outcomes for retrieval cycles in src/actor/runtime.go
- [X] T018 [US1] Expose actor-level opt-in batching configuration APIs in src/actor/actor_ref.go

**Checkpoint**: User Story 1 independently functional and testable.

---

## Phase 4: User Story 2 - Optimize I/O with Bulk Operations (Priority: P2)

**Goal**: Enable batching actors to aggregate many messages into fewer downstream bulk operations.

**Independent Test**: Simulate sink actor workload and verify downstream operation count is reduced compared with one-by-one processing while all messages are accounted for.

### Tests for User Story 2 (REQUIRED)

- [X] T019 [P] [US2] Add integration test for single bulk downstream operation from one batch in tests/integration/batching_enables_single_bulk_downstream_operation_test.go
- [X] T020 [P] [US2] Add integration test for reduced downstream call count under burst load in tests/integration/batching_reduces_downstream_operation_count_test.go
- [X] T021 [P] [US2] Add unit test for batch envelope construction metadata in tests/unit/batch_envelope_construction_test.go
- [X] T022 [P] [US2] Add contract assertions for I/O optimization guarantees in tests/contract/mailbox_batching_contract_test.go

### Implementation for User Story 2

- [X] T023 [US2] Implement batch envelope construction and propagation in src/actor/runtime.go
- [X] T024 [US2] Add batch-size/result telemetry fields for optimization observability in src/actor/events.go
- [X] T025 [US2] Add runtime metrics implementation for batch outcomes in src/internal/metrics/runtime_metrics.go

**Checkpoint**: User Stories 1 and 2 both independently functional.

---

## Phase 5: User Story 3 - Preserve Predictable Failure and Recovery (Priority: P3)

**Goal**: Keep supervision/failure behavior explicit and deterministic when batch handlers fail and actors recover.

**Independent Test**: Force batch handler failure and verify deterministic supervision outcomes and resumed batching behavior after recovery.

### Tests for User Story 3 (REQUIRED)

- [X] T026 [P] [US3] Add integration test for batch handler failure supervision outcome in tests/integration/batch_failure_emits_deterministic_supervision_outcome_test.go
- [X] T027 [P] [US3] Add integration test for post-restart batching continuity in tests/integration/batch_processing_resumes_after_restart_test.go
- [X] T028 [P] [US3] Add integration test for invalid batch configuration deterministic handling in tests/integration/invalid_batch_config_is_deterministic_test.go
- [X] T029 [P] [US3] Add contract assertions for batch failure/recovery guarantees in tests/contract/mailbox_batching_contract_test.go

### Implementation for User Story 3

- [X] T030 [US3] Implement batch failure outcome emission and reason-code mapping in src/actor/runtime.go
- [X] T031 [US3] Integrate batch failure paths with existing supervision decision flow in src/actor/runtime.go
- [X] T032 [US3] Persist/recover actor batching configuration across restart cycles in src/actor/runtime.go

**Checkpoint**: All user stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency, docs, and full validation.

- [X] T033 [P] Add/refresh unit tests for batch outcome store queries in tests/unit/batch_outcome_store_test.go
- [X] T034 Update batching API and behavior documentation in README.md
- [X] T035 Validate quickstart commands and expected outcomes in specs/010-mailbox-batching/quickstart.md
- [X] T036 Run full verification suite and record final results in specs/010-mailbox-batching/tasks.md

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (Setup): no dependencies.
- Phase 2 (Foundational): depends on Setup; blocks all user stories.
- Phase 3 (US1): depends on Foundational.
- Phase 4 (US2): depends on Foundational and US1 batch cycle implementation.
- Phase 5 (US3): depends on Foundational, US1 execution semantics, and US2 telemetry coverage.
- Phase 6 (Polish): depends on completion of selected user stories.

### User Story Dependencies

- US1 (P1): first deliverable MVP after foundation.
- US2 (P2): builds on US1 batch retrieval by validating downstream optimization.
- US3 (P3): builds on US1/US2 behavior to guarantee deterministic failure and recovery.

### Within Each User Story

- Write tests first and confirm failure.
- Implement minimum behavior to pass tests.
- Re-run story-specific tests before moving forward.

## Dependency Graph

- Foundation → US1 → US2 → US3
- Polish depends on US1/US2/US3 completion

## Parallel Execution Examples

### User Story 1

```bash
Task: "T010 [US1] in tests/integration/batch_receive_consumes_multiple_messages_per_cycle_test.go"
Task: "T011 [US1] in tests/integration/batch_receive_respects_configured_batch_limit_test.go"
Task: "T012 [US1] in tests/integration/non_batched_actor_behavior_unchanged_test.go"
Task: "T013 [US1] in tests/unit/mailbox_bulk_dequeue_ordering_test.go"
Task: "T014 [US1] in tests/contract/mailbox_batching_contract_test.go"
```

### User Story 2

```bash
Task: "T019 [US2] in tests/integration/batching_enables_single_bulk_downstream_operation_test.go"
Task: "T020 [US2] in tests/integration/batching_reduces_downstream_operation_count_test.go"
Task: "T021 [US2] in tests/unit/batch_envelope_construction_test.go"
Task: "T022 [US2] in tests/contract/mailbox_batching_contract_test.go"
```

### User Story 3

```bash
Task: "T026 [US3] in tests/integration/batch_failure_emits_deterministic_supervision_outcome_test.go"
Task: "T027 [US3] in tests/integration/batch_processing_resumes_after_restart_test.go"
Task: "T028 [US3] in tests/integration/invalid_batch_config_is_deterministic_test.go"
Task: "T029 [US3] in tests/contract/mailbox_batching_contract_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 (US1).
3. Validate batch retrieval behavior independently.
4. Stop for MVP review.

### Incremental Delivery

1. Deliver US1 opt-in bounded batch retrieval.
2. Deliver US2 downstream bulk-operation optimization surface.
3. Deliver US3 deterministic failure/recovery behavior.
4. Finish polish and full-suite validation.

### Parallel Team Strategy

1. Team completes setup + foundations first.
2. One engineer drives US1 runtime batch loop while another prepares US2 tests.
3. After US1 merge, proceed with US2 then US3 per dependency chain.

## Notes

- [P] tasks are parallelizable when they touch different files and do not depend on incomplete tasks.
- All tasks include explicit file paths and are immediately executable.
- Keep batching behavior internal to actor runtime so caller-facing location-transparent contracts remain unchanged.

## Verification Results

- `go test ./tests/integration/... -run TestBatchReceiveConsumesMultipleMessagesPerCycle` ✅
- `go test ./tests/integration/... -run TestBatchReceiveRespectsConfiguredBatchLimit` ✅
- `go test ./tests/integration/... -run TestBatchingEnablesSingleBulkDownstreamOperation` ✅
- `go test ./tests/integration/... -run TestBatchFailureEmitsDeterministicSupervisionOutcome` ✅
- `go test ./tests/contract/... -run TestMailboxBatchingContract` ✅
- `go vet ./...` ✅
- `go test ./...` ✅
- `go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...` ✅
