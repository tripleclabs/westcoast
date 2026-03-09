# Tasks: Native Pub/Sub Event Bus

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/native-pubsub-broker-contract.md

**Tests**: Test tasks are REQUIRED. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Align repository and feature scaffolding for broker/event-bus implementation.

- [X] T001 Create feature task documentation scaffold in /Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/tasks.md
- [X] T002 [P] Add broker test helpers for runtime/bootstrap setup in /Volumes/Store1/src/3clabs/westcoast/tests/integration/pubsub_test_helpers_test.go
- [X] T003 [P] Add shared pubsub fixture builders for unit tests in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pubsub_test_helpers_test.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core broker primitives required before user-story behavior can be implemented.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [X] T004 Define broker command, publish envelope, and subscription types in /Volumes/Store1/src/3clabs/westcoast/src/actor/types.go
- [X] T005 Define broker operation and delivery outcome enums/records in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go
- [X] T006 [P] Add broker event emission types and payload fields in /Volumes/Store1/src/3clabs/westcoast/src/actor/events.go
- [X] T007 Implement topic pattern parser/validator for exact, `+`, and `#` rules in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_pattern.go
- [X] T008 Implement Trie node/index structures and deterministic matcher in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_trie.go
- [X] T009 Implement runtime broker registration/bootstrap lifecycle in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T010 [P] Add unit tests for topic-pattern validation and invalid wildcard placement in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pubsub_topic_pattern_validation_test.go
- [X] T011 [P] Add unit tests for Trie exact/wildcard matching determinism in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pubsub_trie_match_test.go
- [X] T012 Add unit tests for broker outcome classification records in /Volumes/Store1/src/3clabs/westcoast/tests/unit/pubsub_outcomes_unit_test.go

**Checkpoint**: Foundation ready - user story implementation can now begin.

---

## Phase 3: User Story 1 - Publish and Fan-out Events (Priority: P1) 🎯 MVP

**Goal**: Publish events to exact topics and fan out asynchronous copies to all matching subscribers.

**Independent Test**: Register multiple exact-topic subscribers, publish one event, and verify all intended subscribers receive one copy while publisher call flow remains non-blocking.

### Tests for User Story 1 (REQUIRED) ⚠️

- [X] T013 [P] [US1] Add contract test for exact-topic publish fan-out guarantees in /Volumes/Store1/src/3clabs/westcoast/tests/contract/native_pubsub_broker_contract_test.go
- [X] T014 [P] [US1] Add integration test for exact-topic publish to multiple subscribers in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_publish_fanout_exact_topic_test.go
- [X] T015 [P] [US1] Add integration test for no-match publish deterministic completion in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_publish_no_matching_subscribers_test.go

### Implementation for User Story 1

- [X] T016 [US1] Implement broker subscribe index and exact-topic lookup path in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_broker.go
- [X] T017 [US1] Implement asynchronous publish fan-out dispatch with per-target isolation in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_dispatch.go
- [X] T018 [US1] Wire publish command handling into runtime message loop in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T019 [US1] Emit deterministic publish outcomes (`publish_success`, `target_unreachable`, `publish_partial_delivery`) in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: User Story 1 is independently functional and testable.

---

## Phase 4: User Story 2 - Route by Wildcard Topic Patterns (Priority: P2)

**Goal**: Support hierarchical wildcard subscriptions (`+`, `#`) with deterministic segment-based routing.

**Independent Test**: Subscribe to exact, single-segment wildcard, and tail wildcard patterns; publish mixed topics; verify only matching subscribers receive each event.

### Tests for User Story 2 (REQUIRED) ⚠️

- [X] T020 [P] [US2] Add contract coverage for wildcard semantics and tail-wildcard rules in /Volumes/Store1/src/3clabs/westcoast/tests/contract/native_pubsub_broker_contract_test.go
- [X] T021 [P] [US2] Add integration test for single-segment wildcard routing (`user.+.updated`) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_wildcard_single_segment_routing_test.go
- [X] T022 [P] [US2] Add integration test for tail wildcard routing (`audit.#`) in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_wildcard_tail_routing_test.go
- [X] T023 [P] [US2] Add integration test proving non-matching wildcard subscriptions receive no deliveries in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_wildcard_nonmatch_test.go

### Implementation for User Story 2

- [X] T024 [US2] Implement wildcard-aware Trie traversal and deduped PID result set in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_trie.go
- [X] T025 [US2] Enforce wildcard grammar constraints and deterministic invalid-pattern errors in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_pattern.go
- [X] T026 [US2] Integrate wildcard matcher into publish resolution path without duplicate deliveries in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_broker.go

**Checkpoint**: User Stories 1 and 2 are independently functional and testable.

---

## Phase 5: User Story 3 - Manage Subscriptions Dynamically at Runtime (Priority: P3)

**Goal**: Allow actors to subscribe/unsubscribe via Ask commands with deterministic acknowledgements and idempotent behavior.

**Independent Test**: Ask subscribe, publish and verify delivery, Ask unsubscribe, publish again and verify no delivery; validate deterministic no-op/error responses.

### Tests for User Story 3 (REQUIRED) ⚠️

- [X] T027 [P] [US3] Add contract coverage for Ask subscribe/unsubscribe acknowledgement semantics in /Volumes/Store1/src/3clabs/westcoast/tests/contract/native_pubsub_broker_contract_test.go
- [X] T028 [P] [US3] Add integration test for Ask subscribe then publish delivery in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_dynamic_subscribe_via_ask_test.go
- [X] T029 [P] [US3] Add integration test for Ask unsubscribe then no further delivery in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_dynamic_unsubscribe_via_ask_test.go
- [X] T030 [P] [US3] Add integration test for duplicate subscribe idempotency and missing-unsubscribe no-op acknowledgement in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_subscription_idempotency_test.go
- [X] T031 [P] [US3] Add integration test for invalid broker command payload deterministic error response in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_invalid_command_outcome_test.go

### Implementation for User Story 3

- [X] T032 [US3] Implement Ask command handlers for subscribe/unsubscribe in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_commands.go
- [X] T033 [US3] Implement subscription reverse index for deterministic removal/idempotency in /Volumes/Store1/src/3clabs/westcoast/src/actor/pubsub_broker.go
- [X] T034 [US3] Wire broker Ask command routing and reply envelopes in /Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go
- [X] T035 [US3] Emit deterministic subscribe/unsubscribe outcomes and reason codes in /Volumes/Store1/src/3clabs/westcoast/src/actor/outcome.go

**Checkpoint**: All user stories are independently functional and testable.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Hardening, observability verification, and end-to-end validation across all stories.

- [X] T036 [P] Add integration test for fan-out resilience when one subscriber PID is unreachable in /Volumes/Store1/src/3clabs/westcoast/tests/integration/broker_partial_delivery_resilience_test.go
- [X] T037 [P] Add benchmark for publish fan-out matching throughput in /Volumes/Store1/src/3clabs/westcoast/tests/benchmark/pubsub_broker_fanout_benchmark_test.go
- [X] T038 Update runtime metrics hooks for broker publish/match/outcome counters in /Volumes/Store1/src/3clabs/westcoast/src/internal/metrics/hooks.go
- [X] T039 Update quickstart verification commands and broker usage notes in /Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/quickstart.md
- [X] T040 Run full verification suite and record results in /Volumes/Store1/src/3clabs/westcoast/specs/011-native-pubsub-broker/quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies.
- **Phase 2 (Foundational)**: Depends on Phase 1 completion; blocks all user stories.
- **Phase 3 (US1)**: Depends on Phase 2 completion; delivers MVP.
- **Phase 4 (US2)**: Depends on Phase 2 completion and integrates with US1 publish path.
- **Phase 5 (US3)**: Depends on Phase 2 completion and broker command infrastructure from US1.
- **Phase 6 (Polish)**: Depends on completion of target user stories.

### User Story Dependencies

- **US1 (P1)**: Independent after foundational phase; no dependency on US2/US3.
- **US2 (P2)**: Depends on US1 broker publish path and Trie matcher extension.
- **US3 (P3)**: Depends on US1 broker runtime presence; can proceed in parallel with US2 after foundational if team capacity allows.

### Within Each User Story

- Write tests first and verify they fail.
- Implement core broker behavior after failing tests exist.
- Wire runtime integration after behavior-level components compile.
- Re-run story-specific tests before marking story complete.

### Dependency Graph

- `Setup -> Foundational -> US1 -> Polish`
- `Foundational -> US2 -> Polish`
- `Foundational -> US3 -> Polish`

---

## Parallel Execution Examples

### User Story 1

```bash
Task: "T013 Add contract test for exact-topic publish fan-out guarantees in tests/contract/native_pubsub_broker_contract_test.go"
Task: "T014 Add integration test for exact-topic publish to multiple subscribers in tests/integration/broker_publish_fanout_exact_topic_test.go"
Task: "T015 Add integration test for no-match publish deterministic completion in tests/integration/broker_publish_no_matching_subscribers_test.go"
```

### User Story 2

```bash
Task: "T021 Add integration test for single-segment wildcard routing in tests/integration/broker_wildcard_single_segment_routing_test.go"
Task: "T022 Add integration test for tail wildcard routing in tests/integration/broker_wildcard_tail_routing_test.go"
Task: "T023 Add integration test proving non-matching wildcard subscriptions receive no deliveries in tests/integration/broker_wildcard_nonmatch_test.go"
```

### User Story 3

```bash
Task: "T028 Add integration test for Ask subscribe then publish delivery in tests/integration/broker_dynamic_subscribe_via_ask_test.go"
Task: "T029 Add integration test for Ask unsubscribe then no further delivery in tests/integration/broker_dynamic_unsubscribe_via_ask_test.go"
Task: "T031 Add integration test for invalid broker command payload deterministic error response in tests/integration/broker_invalid_command_outcome_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 (US1).
3. Validate exact-topic publish fan-out behavior with contract and integration tests.
4. Demo/deploy MVP.

### Incremental Delivery

1. Deliver US1 (exact-topic fan-out).
2. Add US2 wildcard routing without regressing US1.
3. Add US3 dynamic runtime subscription management.
4. Finish polish, metrics, and throughput validation.

### Parallel Team Strategy

1. Team completes Setup and Foundational together.
2. One engineer drives US2 wildcard matcher while another drives US3 Ask command flow after US1 baseline broker path is stable.
3. Consolidate in Phase 6 with full contract/integration/benchmark validation.

---

## Notes

- [P] tasks are isolated by file and can run concurrently.
- [US#] labels map every user-story task to traceable spec outcomes.
- Keep publish and Ask contracts location-transparent and PID-based.
- Do not mark tasks complete until related tests pass.
