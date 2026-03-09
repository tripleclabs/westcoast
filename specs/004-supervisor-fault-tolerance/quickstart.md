# Quickstart: Validate Supervisor Trees & Fault Tolerance

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestActorPanicIsolation
go test ./tests/integration/... -run TestCleanStateRestart
go test ./tests/integration/... -run TestMailboxPreservationAcrossRestart
```

Expected: tests fail before implementation changes.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Actor panic interception at message-processing boundary
- Deterministic supervision decisions (`restart`, `stop`, `escalate`)
- Clean state reset on restart
- Mailbox preservation for unprocessed messages across restart
- Structured failure/recovery telemetry emission

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
go test ./tests/contract/... -run TestSupervisionDefaultDeterminism
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestActorPanicIsolation|TestCleanStateRestart|TestMailboxPreservationAcrossRestart|TestRestartLimitDeterminism'
```

Pass criteria:
- Panic isolation: runtime survives and healthy actors continue processing.
- Clean restarts: restarted actor state equals initial snapshot.
- Mailbox preservation: queued unprocessed messages are retained in FIFO order.
- Deterministic supervision: post-limit behavior matches configured policy in all runs.

## 5. Constitution Alignment Checks
- Go-native runtime path preserved; no external serializer added.
- Let-it-crash semantics are explicit and actor-scoped.
- Test-first workflow executed with failing-then-passing tests.
- Location-transparent contracts are unchanged for callers.
- Recovery behavior is observable and deterministic.

## 6. Expected Outcomes
- Actor failures are isolated and recoverable.
- Restarts are clean and do not propagate corrupted state.
- Message loss does not occur for queued-unprocessed mailbox entries during restart.
