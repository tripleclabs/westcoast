# Quickstart: Validate Lifecycle Management Hooks

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestLifecycleStartHookBlocksFirstMessageUntilInitialized
go test ./tests/integration/... -run TestLifecycleStopHookRunsDuringGracefulShutdown
go test ./tests/contract/... -run TestLifecycleHooksOutcomeContract
```

Expected: tests fail before implementation changes.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Optional Start hook execution before first message processing
- Optional Stop hook execution during graceful shutdown
- Deterministic startup/shutdown failure handling and outcomes
- Lifecycle observability outcomes for success/failure paths

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestLifecycleStartHookBlocksFirstMessageUntilInitialized|TestLifecycleStartHookFailurePreventsRunning|TestLifecycleStopHookRunsDuringGracefulShutdown|TestLifecycleStopHookFailureStillStopsActor'
```

Pass criteria:
- Start hook runs before first message and at most once per runtime instance.
- Startup failure prevents message processing.
- Stop hook executes during graceful shutdown.
- Stop failure still results in terminal stopped status.
- Lifecycle outcomes are emitted deterministically.

## 5. Constitution Alignment Checks
- Go-native runtime path is preserved.
- Let-it-crash failure boundaries remain explicit.
- Test-first flow is executed with failing-then-passing evidence.
- Public lifecycle behavior remains location-transparent.
- Hook execution remains simple and measurable in hot paths.

## 6. Expected Outcomes
- Actor authors can safely initialize resources before message handling.
- Actor authors can reliably release resources during graceful shutdown.
- Operators can distinguish startup vs shutdown failures through outcomes.
