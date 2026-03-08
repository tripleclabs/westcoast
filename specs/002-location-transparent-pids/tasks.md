# Tasks: Location-Transparent PIDs

**Input**: Design documents from `/specs/002-location-transparent-pids/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and baseline scaffolding for PID work

- [X] T001 Create PID source scaffolding files in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid.go and /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T002 Create contract test directory and baseline file in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_contract_test.go
- [X] T003 [P] Create benchmark placeholder for PID resolver latency in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/pid_resolver_latency_test.go
- [X] T004 [P] Add Makefile benchmark target for PID resolver profile in /Volumes/Store1/src/3clabs/westcoast/Makefile
- [X] T005 Confirm Go module/test paths for new PID test packages in /Volumes/Store1/src/3clabs/westcoast/go.mod

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core PID abstractions and invariants required before user-story implementation

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define PID type (`namespace`, `actor_id`, `generation`) and canonical validation in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid.go
- [X] T007 Define closed delivery outcome enum constants in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid.go
- [X] T008 Implement resolver entry model and state transitions in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T009 [P] Add resolver event type constants and payload fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T010 Implement generation increment/restart hooks in runtime lifecycle flow in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T011 [P] Add resolver metrics hooks for lookup timing in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T012 Add foundational contract tests for closed outcome set and PID shape in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_contract_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin in priority order

---

## Phase 3: User Story 1 - Address Actors by PID (Priority: P1) 🎯 MVP

**Goal**: Enable caller interactions using PID-only addressing without process-bound references.

**Independent Test**: Create actors, issue PIDs, deliver messages by PID only, and verify no caller API requires direct actor references.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add unit tests for canonical PID shape validation in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pid_shape_test.go
- [X] T014 [P] [US1] Add integration tests for PID-only message delivery in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_delivery_test.go
- [X] T015 [US1] Add contract tests for PID issuance and resolver lookup behavior in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_contract_test.go

### Implementation for User Story 1

- [X] T016 [P] [US1] Implement PID issuance API for actor identities in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid.go
- [X] T017 [P] [US1] Implement resolver register/resolve operations in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T018 [US1] Implement send-by-PID path in actor reference runtime bridge in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T019 [US1] Enforce no direct reference exposure in caller-facing APIs in /Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go
- [X] T020 [US1] Emit `pid_resolved` and `pid_delivered` events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go

**Checkpoint**: User Story 1 is fully functional and testable independently

---

## Phase 4: User Story 2 - Preserve Stable PID Semantics Through Lifecycle Changes (Priority: P2)

**Goal**: Keep PID semantics deterministic across stop/restart and reject stale generation deliveries.

**Independent Test**: Trigger lifecycle transitions and verify outcomes remain in closed-set contract with stale generation rejected.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T021 [P] [US2] Add integration tests for stopped PID behavior outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_stopped_outcome_test.go
- [X] T022 [P] [US2] Add integration tests for stale generation rejection in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_stale_generation_test.go
- [X] T023 [US2] Add contract tests for closed outcome determinism under restart/stop in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_contract_test.go

### Implementation for User Story 2

- [X] T024 [P] [US2] Implement generation bump on supervised restart in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T025 [P] [US2] Implement stale-generation validation in resolver delivery path in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T026 [US2] Implement closed outcome mapping for stopped/unresolved/not-found states in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T027 [US2] Emit `pid_rejected` and `pid_unresolved` events with canonical outcome labels in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T028 [US2] Persist and expose deterministic delivery outcomes for PID attempts in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Prepare PID Model for Multi-Node Routing (Priority: P3)

**Goal**: Provide forward-compatible resolver seams and performance validation for future distributed routing.

**Independent Test**: Validate resolver abstraction can switch backing strategy without caller contract change and confirm latency gate.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T029 [P] [US3] Add unit tests for resolver interface/backing swap invariants in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pid_resolver_interface_test.go
- [X] T030 [P] [US3] Add integration tests for caller contract stability across resolver backends in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_resolver_swap_test.go
- [X] T031 [US3] Add benchmark test for PID resolver latency p95 gate in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/pid_resolver_latency_test.go

### Implementation for User Story 3

- [X] T032 [P] [US3] Implement resolver interface seam for transport/discovery evolution in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T033 [P] [US3] Implement namespace-aware PID routing index in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T034 [US3] Instrument resolver lookup latency metrics and percentile aggregation hooks in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go
- [X] T035 [US3] Implement benchmark harness for p95 <= 25 µs validation in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/pid_resolver_latency_test.go
- [X] T036 [US3] Validate transport-abstracted resolver contract invariants in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_contract_test.go

**Checkpoint**: All user stories are independently functional and future-routing ready

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final quality, documentation, and regression coverage across stories

- [X] T037 [P] Update PID contract document with final closed outcomes and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/contracts/pid-resolver-contract.md
- [X] T038 [P] Update quickstart commands and baseline profile guidance in /Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/quickstart.md
- [X] T039 Run full test and benchmark verification from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression edge-case tests (unknown PID, retired entry, rapid restarts) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_edge_cases_test.go
- [X] T041 Confirm terminology consistency (`PID`, `generation`, closed outcomes) in /Volumes/Store1/src/3clabs/westcoast/specs/002-location-transparent-pids/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of targeted user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 PID issuance/resolution base behavior
- **User Story 3 (P3)**: Depends on US1+US2 semantics and closed outcomes

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Resolver/data-model primitives before routing behavior integration
- Contract and integration checks before story completion

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
Task: "T013 [US1] canonical PID shape tests in tests/unit/pid_shape_test.go"
Task: "T014 [US1] PID-only delivery integration tests in tests/integration/pid_delivery_test.go"
Task: "T016 [US1] PID issuance API in src/actor/pid.go"
Task: "T017 [US1] resolver register/resolve in src/actor/pid_resolver.go"
```

## Parallel Example: User Story 2

```bash
Task: "T021 [US2] stopped PID outcome tests in tests/integration/pid_stopped_outcome_test.go"
Task: "T022 [US2] stale generation rejection tests in tests/integration/pid_stale_generation_test.go"
Task: "T024 [US2] generation bump on restart in src/actor/runtime.go"
Task: "T025 [US2] stale-generation validator in src/actor/pid_resolver.go"
```

## Parallel Example: User Story 3

```bash
Task: "T029 [US3] resolver seam unit tests in tests/unit/pid_resolver_interface_test.go"
Task: "T030 [US3] resolver swap integration tests in tests/integration/pid_resolver_swap_test.go"
Task: "T032 [US3] resolver interface seam in src/actor/pid_resolver.go"
Task: "T033 [US3] namespace routing index in src/actor/pid_resolver.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (PID issuance + PID-based delivery)
2. Deliver US2 (restart/stale-generation deterministic semantics)
3. Deliver US3 (resolver seam + latency gate)
4. Finish with cross-cutting polish and regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split by story with shared contract coverage checkpoints
3. Recombine for Phase 6 final verification and documentation updates
