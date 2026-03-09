# Tasks: Distributed-Ready Guardrails

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare PID-guardrail and gateway-seam test/runtime scaffolding.

- [X] T001 Create PID guardrail integration test helper scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_guardrails_test_helpers_test.go
- [X] T002 Create PID gateway contract test scaffold in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_gateway_guardrails_contract_test.go
- [X] T003 [P] Create unit test scaffold for PID policy invariants in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pid_guardrails_policy_unit_test.go
- [X] T004 [P] Confirm PID delivery touchpoints for policy insertion in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T005 Confirm PID resolver touchpoints for gateway seam in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core policy and gateway-boundary primitives required before user stories.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define PID interaction policy and gateway mode enums in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T007 Define policy and gateway observability event types/fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T008 Implement PID policy enforcement entry primitives in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T009 [P] Implement gateway-boundary route abstraction primitives in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T010 [P] Add policy/gateway outcome recording primitives in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T011 Add policy/gateway metric hook interfaces and nop implementations in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T012 Add foundational contract tests for required guardrail outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_gateway_guardrails_contract_test.go
- [X] T013 Add shared helper assertions for policy accept/reject and gateway route outcomes in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_guardrails_test_helpers_test.go

**Checkpoint**: Foundation ready - user stories can proceed in priority order

---

## Phase 3: User Story 1 - Enforce PID-Only Actor Interactions (Priority: P1) 🎯 MVP

**Goal**: Ensure all cross-actor interactions are PID-based and non-PID attempts are deterministically rejected.

**Independent Test**: Attempt compliant PID and non-compliant non-PID cross-actor flows; verify accept/reject outcomes and unchanged behavior for compliant paths.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T014 [P] [US1] Add integration test for non-PID cross-actor rejection in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_policy_rejects_non_pid_test.go
- [X] T015 [P] [US1] Add integration test for PID-compliant behavior equivalence in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pid_policy_compliant_flow_equivalence_test.go
- [X] T016 [US1] Add contract test for PID policy accept/reject semantics in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_gateway_guardrails_contract_test.go

### Implementation for User Story 1

- [X] T017 [P] [US1] Implement PID-only policy checks in cross-actor dispatch path in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T018 [P] [US1] Expose strict PID interaction policy controls in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T019 [US1] Emit policy_accept/policy_reject_non_pid events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T020 [US1] Record policy decision outcomes for diagnostics in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T021 [US1] Add policy decision metric observations in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go

**Checkpoint**: User Story 1 is fully functional and independently testable

---

## Phase 4: User Story 2 - Preserve Gateway Insertion Seam (Priority: P2)

**Goal**: Provide pluggable gateway-boundary routing seam that keeps business actor behavior topology-agnostic.

**Independent Test**: Run identical PID flows through local-direct and gateway-mediated modes and verify business actor behavior/message semantics remain consistent.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T022 [P] [US2] Add integration test for gateway seam preserving business behavior in /Volumes/Store1/src/3clabs/westcoast/tests/integration/gateway_seam_preserves_behavior_test.go
- [X] T023 [P] [US2] Add integration test for deterministic gateway routing failure outcome in /Volumes/Store1/src/3clabs/westcoast/tests/integration/gateway_seam_route_failure_deterministic_test.go
- [X] T024 [US2] Add contract test for gateway boundary route-mode semantics in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_gateway_guardrails_contract_test.go

### Implementation for User Story 2

- [X] T025 [P] [US2] Implement local_direct and gateway_mediated route selection seam in /Volumes/Store1/src/3clabs/westcoast/src/actor/pid_resolver.go
- [X] T026 [P] [US2] Integrate gateway-boundary seam into PID dispatch workflow in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T027 [US2] Emit gateway_route_success/gateway_route_failure events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T028 [US2] Record gateway route outcomes for diagnostics in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T029 [US2] Add gateway route-mode metrics observations in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go

**Checkpoint**: User Stories 1 and 2 are independently functional and contract-consistent

---

## Phase 5: User Story 3 - Operational Confidence for Future Distribution (Priority: P3)

**Goal**: Ensure guardrail outcomes and readiness validation signals provide release-by-release distributed-readiness evidence.

**Independent Test**: Execute guardrail validation scenarios and verify explicit outcomes for policy/gateway scopes and readiness pass/fail evidence.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T030 [P] [US3] Add integration test for guardrail observability evidence completeness in /Volumes/Store1/src/3clabs/westcoast/tests/integration/guardrail_outcomes_evidence_complete_test.go
- [X] T031 [P] [US3] Add integration test for readiness validation fail-then-pass lifecycle in /Volumes/Store1/src/3clabs/westcoast/tests/integration/readiness_validation_fail_pass_test.go
- [X] T032 [US3] Add contract test for readiness validation scope requirements in /Volumes/Store1/src/3clabs/westcoast/tests/contract/pid_gateway_guardrails_contract_test.go

### Implementation for User Story 3

- [X] T033 [P] [US3] Implement readiness validation record generation in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T034 [P] [US3] Implement release-readiness validation entrypoint in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T035 [US3] Emit validation scope pass/fail events in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T036 [US3] Add metrics observation for guardrail validation scopes in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/runtime_metrics.go

**Checkpoint**: All user stories are independently functional with distributed-readiness evidence

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final regression, docs alignment, and compatibility validation.

- [X] T037 [P] Update guardrail contract wording and invariants in /Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/contracts/pid-gateway-guardrails-contract.md
- [X] T038 [P] Update quickstart verification commands for policy/gateway scenarios in /Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/quickstart.md
- [X] T039 Run full verification suite (unit/integration/contract) from /Volumes/Store1/src/3clabs/westcoast/tests
- [X] T040 [P] Add regression test for mixed local-direct and gateway-mediated traffic determinism in /Volumes/Store1/src/3clabs/westcoast/tests/integration/gateway_mixed_mode_determinism_test.go
- [X] T041 Confirm location-transparent guardrail consistency in /Volumes/Store1/src/3clabs/westcoast/specs/007-pid-gateway-guardrails/spec.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; can start immediately
- **Foundational (Phase 2)**: Depends on Setup; blocks all user stories
- **User Stories (Phase 3+)**: Depend on Foundational completion
- **Polish (Phase 6)**: Depends on completion of all user stories

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Foundational; no dependency on other stories
- **User Story 2 (P2)**: Depends on US1 PID policy baseline
- **User Story 3 (P3)**: Depends on US1+US2 outcomes and gateway seam behavior

### Dependency Graph

- `US1 -> US2 -> US3`
- `US1` is the recommended MVP release slice

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Policy/routing primitives before event/metrics wiring
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
Task: "T014 [US1] non-PID rejection integration test in tests/integration/pid_policy_rejects_non_pid_test.go"
Task: "T015 [US1] PID equivalence integration test in tests/integration/pid_policy_compliant_flow_equivalence_test.go"
Task: "T017 [US1] PID policy runtime checks in src/actor/runtime.go"
Task: "T018 [US1] strict PID policy types in src/actor/types.go"
```

## Parallel Example: User Story 2

```bash
Task: "T022 [US2] gateway seam behavior preservation integration test in tests/integration/gateway_seam_preserves_behavior_test.go"
Task: "T023 [US2] deterministic gateway route failure test in tests/integration/gateway_seam_route_failure_deterministic_test.go"
Task: "T025 [US2] route mode seam in src/actor/pid_resolver.go"
Task: "T026 [US2] runtime gateway seam integration in src/actor/runtime.go"
```

## Parallel Example: User Story 3

```bash
Task: "T030 [US3] guardrail evidence completeness integration test in tests/integration/guardrail_outcomes_evidence_complete_test.go"
Task: "T031 [US3] readiness fail-pass lifecycle integration test in tests/integration/readiness_validation_fail_pass_test.go"
Task: "T033 [US3] readiness record generation in src/actor/outcome.go"
Task: "T034 [US3] readiness validation runtime entrypoint in src/actor/runtime.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1
4. Validate US1 independently before broader rollout

### Incremental Delivery

1. Deliver US1 (PID-only policy guardrails)
2. Deliver US2 (gateway seam compatibility)
3. Deliver US3 (operational readiness evidence)
4. Execute polish and full regression validation

### Parallel Team Strategy

1. Complete Setup + Foundational together
2. Split by story once foundation is complete
3. Recombine for final regression/docs/consistency completion
