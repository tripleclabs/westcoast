# Quickstart: Validate Location-Transparent PIDs

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write and Run Failing Tests First (Red)

```bash
go test ./tests/unit/... -run TestPIDCanonicalShape
go test ./tests/integration/... -run TestPIDStaleGenerationRejected
go test ./tests/integration/... -run TestPIDClosedOutcomeSet
```

Expected: tests fail before implementation.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- PID issuance using canonical shape (`namespace + actor_id + generation`)
- Resolver boundary for PID resolution
- Closed outcome mapping for every delivery attempt
- Generation increment and stale-generation rejection on restart
- PID resolution and delivery event emission

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/benchmark -run '^$' -bench BenchmarkPIDResolverLatency -benchmem -benchtime=3s
```

## 4. Validate Performance Gate

```bash
WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$' -bench BenchmarkPIDResolverLatency -benchmem -benchtime=3s
```

Pass criterion:
- Resolver lookup p95 <= 25 µs on baseline profile.

## 5. Constitution Alignment Checks
- Public interactions use only PID contracts (no pointer/local reference exposure).
- Failure outcomes are explicit and contract-bounded.
- Restart semantics enforce generation-based stale message rejection.
- Observability includes resolver and delivery outcomes.

## 6. Expected Outcomes
- PID API is the single caller-facing addressing mechanism.
- Delivery outcomes always belong to the closed set.
- Restarted actors require current generation; stale generation attempts are rejected.
- Resolver latency benchmark satisfies p95 <= 25 µs gate.
