# Tasks: Actor Routers and Worker Pools

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/009-actor-router-pools/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/router-worker-pool-contract.md, quickstart.md

**Tests**: Test tasks are REQUIRED for this feature. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story for independent implementation and validation.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare router test scaffolding and shared interfaces.

- [X] T001 Create shared router test helpers in tests/integration/router_test_helpers_test.go
- [X] T002 Create contract test scaffold for router behavior in tests/contract/router_worker_pool_contract_test.go
- [X] T003 [P] Add router outcome metric hook placeholders in src/internal/metrics/hooks.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build common router/pool primitives required by all user stories.

**⚠️ CRITICAL**: Complete this phase before user story work.

- [X] T004 Define router strategy and shard key domain types in src/actor/types.go
- [X] T005 Add router lifecycle/routing event types and fields in src/actor/events.go
- [X] T006 Implement router outcome storage/query primitives in src/actor/outcome.go
- [X] T007 Add runtime router registry and pool configuration state in src/actor/runtime.go
- [X] T008 [P] Add actor reference surfaces for router send/config operations in src/actor/actor_ref.go

**Checkpoint**: Foundation complete; user stories can proceed.

---

## Phase 3: User Story 1 - Stateless Pool Routing (Priority: P1) 🎯 MVP

**Goal**: Route incoming messages across worker pools using round-robin and random strategies.

**Independent Test**: Configure router with multiple workers and verify round-robin/random distributions while all messages process successfully.

### Tests for User Story 1 (REQUIRED)

- [X] T009 [P] [US1] Add integration test for round-robin distribution in tests/integration/router_round_robin_distributes_across_workers_test.go
- [X] T010 [P] [US1] Add integration test for random distribution in tests/integration/router_random_strategy_distributes_across_workers_test.go
- [X] T011 [P] [US1] Add unit test for concurrent round-robin index behavior in tests/unit/router_round_robin_counter_test.go
- [X] T012 [P] [US1] Add contract assertions for stateless routing outcomes in tests/contract/router_worker_pool_contract_test.go

### Implementation for User Story 1

- [X] T013 [US1] Implement round-robin router dispatch path in src/actor/runtime.go
- [X] T014 [US1] Implement random router dispatch path in src/actor/runtime.go
- [X] T015 [US1] Emit stateless routing success/failure outcomes in src/actor/runtime.go

**Checkpoint**: User Story 1 independently functional and testable.

---

## Phase 4: User Story 2 - Consistent Hash Sharding (Priority: P2)

**Goal**: Route keyed messages to stable worker targets using consistent hash mapping.

**Independent Test**: Same-key messages always route to same worker for stable pool; invalid/missing keys fail deterministically.

### Tests for User Story 2 (REQUIRED)

- [X] T016 [P] [US2] Add integration test for same-key affinity in tests/integration/consistent_hash_routes_same_key_to_same_worker_test.go
- [X] T017 [P] [US2] Add integration test for different-key spread in tests/integration/consistent_hash_distributes_different_keys_test.go
- [X] T018 [P] [US2] Add integration test for missing/invalid key rejection in tests/integration/router_rejects_consistent_hash_message_without_key_test.go
- [X] T019 [P] [US2] Add contract assertions for consistent-hash guarantees in tests/contract/router_worker_pool_contract_test.go

### Implementation for User Story 2

- [X] T020 [US2] Implement shard-key extraction contract handling in src/actor/types.go
- [X] T021 [US2] Implement consistent-hash worker selection logic in src/actor/runtime.go
- [X] T022 [US2] Emit deterministic invalid-key routing outcomes in src/actor/runtime.go

**Checkpoint**: User Stories 1 and 2 both independently functional.

---

## Phase 5: User Story 3 - Router Fault and Pool Recovery Behavior (Priority: P3)

**Goal**: Preserve deterministic and observable routing behavior under worker/router failures.

**Independent Test**: Simulate worker failure and recovery; verify explicit outcomes and resumed strategy behavior.

### Tests for User Story 3 (REQUIRED)

- [X] T023 [P] [US3] Add integration test for zero-worker failure outcome in tests/integration/router_zero_worker_pool_rejected_test.go
- [X] T024 [P] [US3] Add integration test for unavailable worker deterministic outcome in tests/integration/router_worker_failure_produces_deterministic_outcome_test.go
- [X] T025 [P] [US3] Add integration test for post-recovery routing continuation in tests/integration/router_pool_recovery_resumes_routing_test.go
- [X] T026 [P] [US3] Add contract assertions for router fault/recovery outcomes in tests/contract/router_worker_pool_contract_test.go

### Implementation for User Story 3

- [X] T027 [US3] Implement zero-worker and unavailable-worker failure branches in src/actor/runtime.go
- [X] T028 [US3] Integrate router dispatch with existing supervision-compatible failure surfaces in src/actor/runtime.go
- [X] T029 [US3] Add router outcome metrics implementation for failure classes in src/internal/metrics/runtime_metrics.go

**Checkpoint**: All user stories independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final consistency, docs, and full validation.

- [X] T030 [P] Add/refresh unit tests for router outcome store querying in tests/unit/router_outcome_store_test.go
- [X] T031 Update router usage and strategy behavior docs in README.md
- [X] T032 Validate quickstart commands and expected outcomes in specs/009-actor-router-pools/quickstart.md
- [X] T033 Run full verification suite and record final results in specs/009-actor-router-pools/tasks.md

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (Setup): no dependencies.
- Phase 2 (Foundational): depends on Setup; blocks all user stories.
- Phase 3 (US1): depends on Foundational.
- Phase 4 (US2): depends on Foundational and US1 router dispatch base.
- Phase 5 (US3): depends on Foundational, US1 dispatch, and US2 hash-key routing.
- Phase 6 (Polish): depends on completion of selected user stories.

### User Story Dependencies

- US1 (P1): first deliverable MVP after foundation.
- US2 (P2): adds deterministic shard affinity on top of base routing.
- US3 (P3): adds fault/recovery guarantees across router/pool operations.

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
Task: "T009 [US1] in tests/integration/router_round_robin_distributes_across_workers_test.go"
Task: "T010 [US1] in tests/integration/router_random_strategy_distributes_across_workers_test.go"
Task: "T011 [US1] in tests/unit/router_round_robin_counter_test.go"
Task: "T012 [US1] in tests/contract/router_worker_pool_contract_test.go"
```

### User Story 2

```bash
Task: "T016 [US2] in tests/integration/consistent_hash_routes_same_key_to_same_worker_test.go"
Task: "T017 [US2] in tests/integration/consistent_hash_distributes_different_keys_test.go"
Task: "T018 [US2] in tests/integration/router_rejects_consistent_hash_message_without_key_test.go"
Task: "T019 [US2] in tests/contract/router_worker_pool_contract_test.go"
```

### User Story 3

```bash
Task: "T023 [US3] in tests/integration/router_zero_worker_pool_rejected_test.go"
Task: "T024 [US3] in tests/integration/router_worker_failure_produces_deterministic_outcome_test.go"
Task: "T025 [US3] in tests/integration/router_pool_recovery_resumes_routing_test.go"
Task: "T026 [US3] in tests/contract/router_worker_pool_contract_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 (US1).
3. Validate round-robin/random routing independently.
4. Stop for MVP review.

### Incremental Delivery

1. Deliver US1 stateless routing.
2. Deliver US2 consistent-hash sharding.
3. Deliver US3 fault/recovery behavior.
4. Finish polish and full-suite validation.

### Parallel Team Strategy

1. Team completes setup + foundations first.
2. One engineer drives US1 runtime path while another prepares US2 tests.
3. After US1 merge, proceed with US2 then US3 sequential dependency chain.

## Notes

- [P] tasks are parallelizable when they touch different files and do not depend on incomplete tasks.
- All tasks include explicit file paths and are immediately executable.
- Keep router-facing contracts location-transparent to preserve future distributed evolution.

## Verification Results

- `GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test ./...` ✅
- `GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go vet ./...` ✅
- `GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...` ✅
