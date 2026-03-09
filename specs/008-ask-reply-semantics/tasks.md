# Tasks: Request-Response Ask Semantics

**Input**: Design documents from `/Volumes/Store1/src/3clabs/westcoast/specs/008-ask-reply-semantics/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/ask-reply-contract.md, quickstart.md

**Tests**: Test tasks are REQUIRED for this feature. Write tests first and confirm they fail before implementation.

**Organization**: Tasks are grouped by user story so each story can be implemented and tested independently.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare runtime surfaces and test scaffolding for Ask semantics.

- [X] T001 Create Ask feature test helper scaffold in tests/integration/ask_test_helpers_test.go
- [X] T002 Add Ask contract test file scaffold in tests/contract/ask_reply_contract_test.go
- [X] T003 [P] Add Ask outcome metric placeholders in src/internal/metrics/hooks.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build shared primitives used by all Ask user stories.

**⚠️ CRITICAL**: No user story implementation starts before this phase is complete.

- [X] T004 Define Ask request/reply domain types in src/actor/types.go
- [X] T005 Add Ask lifecycle event types and event fields in src/actor/events.go
- [X] T006 Implement Ask outcome storage/query primitives in src/actor/outcome.go
- [X] T007 Add Ask-capable runtime internal state (pending wait slots, correlation index, cleanup path) in src/actor/runtime.go
- [X] T008 [P] Expose Ask API signatures on actor references in src/actor/actor_ref.go

**Checkpoint**: Foundation ready - user story work can begin.

---

## Phase 3: User Story 1 - Ask With Timeout (Priority: P1) 🎯 MVP

**Goal**: Caller can issue Ask and deterministically receive reply-or-timeout.

**Independent Test**: Issue Ask to responder actor; verify successful reply before timeout and timeout error when no reply arrives.

### Tests for User Story 1 (REQUIRED)

- [X] T009 [P] [US1] Add integration test for Ask success path in tests/integration/ask_returns_reply_before_timeout_test.go
- [X] T010 [P] [US1] Add integration test for Ask timeout path in tests/integration/ask_times_out_when_no_reply_test.go
- [X] T011 [P] [US1] Add unit test for Ask waiter timeout/cancel transitions in tests/unit/ask_wait_slot_state_test.go
- [X] T012 [P] [US1] Add contract assertions for terminal Ask outcomes in tests/contract/ask_reply_contract_test.go

### Implementation for User Story 1

- [X] T013 [US1] Implement Runtime Ask call path with timeout-bounded wait completion in src/actor/runtime.go
- [X] T014 [US1] Implement Ask terminal outcome emission for success/timeout/canceled in src/actor/runtime.go
- [X] T015 [US1] Wire Ask API entrypoints through ActorRef in src/actor/actor_ref.go

**Checkpoint**: User Story 1 is independently functional and testable.

---

## Phase 4: User Story 2 - Implicit Reply Routing Context (Priority: P2)

**Goal**: Ask-originated messages carry implicit ReplyTo context and correlation metadata.

**Independent Test**: Receiver extracts ReplyTo from Ask message context and responds through it to the original waiting caller.

### Tests for User Story 2 (REQUIRED)

- [X] T016 [P] [US2] Add integration test for implicit ReplyTo injection in tests/integration/ask_message_includes_implicit_replyto_pid_test.go
- [X] T017 [P] [US2] Add integration test for request correlation uniqueness under concurrency in tests/integration/ask_concurrent_requests_do_not_cross_wire_test.go
- [X] T018 [P] [US2] Add contract test for required ask context fields (`request_id`, `reply_to`) in tests/contract/ask_reply_contract_test.go

### Implementation for User Story 2

- [X] T019 [US2] Attach Ask request context (`request_id`, `reply_to`) to Ask-originated messages in src/actor/runtime.go
- [X] T020 [US2] Add message context accessors for Ask reply metadata in src/actor/types.go
- [X] T021 [US2] Implement reply correlation validation and one-reply completion guard in src/actor/runtime.go

**Checkpoint**: User Stories 1 and 2 both work independently.

---

## Phase 5: User Story 3 - Asynchronous Delegation via ReplyTo (Priority: P3)

**Goal**: Receiver can defer long-running work and reply later via ReplyTo without blocking actor loop.

**Independent Test**: Receiver delegates long-running work off-loop, continues processing other messages, and later delivers deferred reply via ReplyTo.

### Tests for User Story 3 (REQUIRED)

- [X] T022 [P] [US3] Add integration test for deferred async reply behavior in tests/integration/responder_can_reply_asynchronously_via_replyto_test.go
- [X] T023 [P] [US3] Add integration test that actor loop remains responsive during deferred reply in tests/integration/ask_async_delegation_keeps_actor_responsive_test.go
- [X] T024 [P] [US3] Add integration test for late reply drop after timeout in tests/integration/late_reply_is_dropped_after_timeout_test.go
- [X] T025 [P] [US3] Add contract test for `ask_late_reply_dropped` auxiliary outcome in tests/contract/ask_reply_contract_test.go

### Implementation for User Story 3

- [X] T026 [US3] Implement deferred reply acceptance via ReplyTo PID send path in src/actor/runtime.go
- [X] T027 [US3] Implement late-reply rejection/drop logic with deterministic outcome emission in src/actor/runtime.go
- [X] T028 [US3] Add Ask outcome metrics hooks for deferred/late-reply cases in src/internal/metrics/runtime_metrics.go

**Checkpoint**: All user stories are independently functional.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finish feature-level quality and documentation across stories.

- [X] T029 [P] Add/refresh unit tests for Ask outcome store retention and query behavior in tests/unit/ask_outcome_store_test.go
- [X] T030 Update quick usage docs for Ask and async reply flow in README.md
- [X] T031 Validate quickstart scenario commands and expected outcomes in specs/008-ask-reply-semantics/quickstart.md
- [X] T032 Run full verification suite and record results in specs/008-ask-reply-semantics/tasks.md

---

## Dependencies & Execution Order

### Phase Dependencies

- Phase 1 (Setup): no dependencies.
- Phase 2 (Foundational): depends on Phase 1 and blocks all user stories.
- Phase 3 (US1): depends on Phase 2.
- Phase 4 (US2): depends on Phase 2 and US1 runtime Ask path.
- Phase 5 (US3): depends on Phase 2 and US2 reply-context/correlation logic.
- Phase 6 (Polish): depends on completion of selected user stories.

### User Story Dependencies

- US1 (P1): standalone after foundation; delivers MVP Ask value.
- US2 (P2): depends on US1 Ask path and adds implicit ReplyTo/correlation guarantees.
- US3 (P3): depends on US2 reply context and adds deferred reply behavior.

### Within Each User Story

- Write tests first and confirm failure.
- Implement runtime/types changes.
- Re-run tests for the story before advancing.

## Dependency Graph

- Foundation → US1 → US2 → US3
- Polish depends on US1/US2/US3 completion

## Parallel Execution Examples

### User Story 1

```bash
# Parallel test authoring
Task: "T009 [US1] in tests/integration/ask_returns_reply_before_timeout_test.go"
Task: "T010 [US1] in tests/integration/ask_times_out_when_no_reply_test.go"
Task: "T011 [US1] in tests/unit/ask_wait_slot_state_test.go"
Task: "T012 [US1] in tests/contract/ask_reply_contract_test.go"
```

### User Story 2

```bash
# Parallel test authoring before implementation
Task: "T016 [US2] in tests/integration/ask_message_includes_implicit_replyto_pid_test.go"
Task: "T017 [US2] in tests/integration/ask_concurrent_requests_do_not_cross_wire_test.go"
Task: "T018 [US2] in tests/contract/ask_reply_contract_test.go"
```

### User Story 3

```bash
# Parallel test authoring for deferred and late-reply behavior
Task: "T022 [US3] in tests/integration/responder_can_reply_asynchronously_via_replyto_test.go"
Task: "T023 [US3] in tests/integration/ask_async_delegation_keeps_actor_responsive_test.go"
Task: "T024 [US3] in tests/integration/late_reply_is_dropped_after_timeout_test.go"
Task: "T025 [US3] in tests/contract/ask_reply_contract_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1 and Phase 2.
2. Complete Phase 3 (US1).
3. Validate Ask success/timeout behavior independently.
4. Stop for MVP review.

### Incremental Delivery

1. Deliver US1 (Ask timeout-bounded request-response).
2. Deliver US2 (implicit ReplyTo and correlation safety).
3. Deliver US3 (async delegation + late-reply handling).
4. Finish polish and full-suite validation.

### Parallel Team Strategy

1. One engineer handles foundational runtime/type/event primitives.
2. After foundation, one engineer focuses US2 tests while another prepares US3 tests.
3. Merge sequentially by dependency: US1 runtime path first, then US2 correlation, then US3 deferred behavior.

## Notes

- [P] tasks are safe to run in parallel when they target different files with no incomplete dependencies.
- All tasks include explicit file paths and are immediately executable by an LLM agent.
- Keep public Ask/reply contracts PID-based to preserve location transparency for future distributed routing.
- Verification log (2026-03-09):
  - `go test ./...` passed
  - `go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...` passed
  - `go vet ./...` passed
