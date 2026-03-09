# Quickstart: Validate Ask Semantics and Asynchronous Replies

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestAskReturnsReplyBeforeTimeout
go test ./tests/integration/... -run TestAskTimesOutWhenNoReply
go test ./tests/integration/... -run TestAskMessageIncludesImplicitReplyToPID
go test ./tests/integration/... -run TestResponderCanReplyAsynchronouslyViaReplyTo
go test ./tests/contract/... -run TestAskReplyContract
```

Expected: tests fail before implementation changes.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Ask interface with timeout-bounded wait lifecycle
- Implicit Ask request context and ReplyTo PID propagation
- Reply correlation and one-terminal-outcome guarantees
- Deferred reply support via ReplyTo in background/delegated execution
- Deterministic handling for timeout, cancellation, and late replies

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestAskReturnsReplyBeforeTimeout|TestAskTimesOutWhenNoReply|TestAskConcurrentRequestsDoNotCrossWireReplies|TestResponderCanReplyAsynchronouslyViaReplyTo|TestLateReplyIsDroppedAfterTimeout'
```

Pass criteria:
- Ask completes with deterministic success/timeout behavior.
- ReplyTo context is present and routable for Ask-originated requests.
- Concurrent Ask calls do not cross-wire responses.
- Asynchronous delegated replies work without blocking actor loop progress.
- Late replies after timeout are dropped/ignored with observable outcome.

## 5. Constitution Alignment Checks
- Go-native runtime implementation remains in-process and lightweight.
- Failure handling and supervision boundaries remain explicit.
- Test-first evidence exists for each Ask behavior change.
- Ask/reply contract stays PID-based and location-transparent.
- Ask wait-state lifecycle remains bounded and operationally observable.

## 6. Expected Outcomes
- Callers can safely use request-response semantics with bounded waiting.
- Responders can defer long work while maintaining loop responsiveness.
- Ask contract can evolve to multi-node transport without caller-facing rewrites.

## Validation Log

- Validation date: 2026-03-09
- Command results:
  - `go test ./tests/integration/... -run 'TestAsk|TestResponderCanReplyAsynchronouslyViaReplyTo|TestLateReplyIsDroppedAfterTimeout'` passed
  - `go test ./tests/unit/... -run 'TestAsk'` passed
  - `go test ./tests/contract/... -run 'TestAskReplyContract'` passed
