# Quickstart: Validate Smart Mailboxes and Message Batching

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestBatchReceiveConsumesMultipleMessagesPerCycle
go test ./tests/integration/... -run TestBatchReceiveRespectsConfiguredBatchLimit
go test ./tests/integration/... -run TestBatchingEnablesSingleBulkDownstreamOperation
go test ./tests/integration/... -run TestBatchFailureEmitsDeterministicSupervisionOutcome
go test ./tests/contract/... -run TestMailboxBatchingContract
```

Expected before implementation: failing tests that capture missing batch capability.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Opt-in actor batch mailbox configuration
- Bounded per-cycle batch dequeue
- Batch execution/outcome telemetry
- Compatibility path for non-batching actors
- Failure/supervision behavior for batch-handler errors

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestBatchReceiveConsumesMultipleMessagesPerCycle|TestBatchingEnablesSingleBulkDownstreamOperation|TestBatchFailureEmitsDeterministicSupervisionOutcome'
```

Pass criteria:
- Batched actor cycles consume multiple messages under queue load.
- Batching reduces downstream call count for sink actor scenario.
- Batch failure outcomes and supervision decisions are explicit and deterministic.
- Non-batching actors keep existing behavior.

## 5. Constitution Alignment Checks
- Runtime implementation remains Go-native and lightweight.
- Batch failures remain explicit and supervision-compatible.
- Failing-then-passing tests prove behavior changes.
- Actor addressing contracts remain location-transparent.
- Batch bounds keep hot-path complexity measurable and simple.

## 6. Expected Outcomes
- High-throughput actors process queued streams with fewer execution cycles.
- Sink actors can aggregate writes into bulk operations.
- Caller-facing contracts remain unchanged and ready for future distributed evolution.
