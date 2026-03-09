# Quickstart: Validate Local Actor Registry & Discovery

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/integration/... -run TestNamedRegistrationUnique
go test ./tests/integration/... -run TestLookupByNameReturnsPID
go test ./tests/integration/... -run TestLifecycleSyncRemovesRegistryEntry
```

Expected: tests fail before implementation changes.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Name registration with uniqueness validation
- Name-to-PID lookup with deterministic not-found outcome
- Lifecycle-synced unregister on graceful/permanent terminal actor stop
- Observable registry operation outcomes

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestNamedRegistrationUnique|TestLookupByNameReturnsPID|TestLifecycleSyncRemovesRegistryEntry|TestConcurrentRegistryDeterminism'
```

Pass criteria:
- Valid unique names register successfully.
- Duplicate names are rejected deterministically.
- Lookups return PID for hits and explicit not-found for misses.
- Terminal actor lifecycle events remove registry entries automatically.

## 5. Constitution Alignment Checks
- Go-native in-memory runtime path preserved.
- Let-it-crash and supervision semantics remain explicit and isolated.
- Test-first workflow executed with failing-then-passing evidence.
- Named discovery contracts remain location-transparent.
- Registry behavior remains deterministic under concurrency.

## 6. Expected Outcomes
- Actors can be discovered dynamically via human-readable names.
- Name-to-PID lookups are deterministic and race-safe.
- Registry entries remain in sync with actor terminal lifecycle events.
