# Quickstart: Validate Type-Agnostic Local Messaging

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Write Failing Tests First (Red)

```bash
go test ./tests/unit/... -run TestTypeRoutingPrecedence
go test ./tests/integration/... -run TestRejectUnsupportedType
go test ./tests/integration/... -run TestRejectVersionMismatch
```

Expected: tests fail before implementation.

## 2. Implement Minimum Feature Slice (Green)

Implement:
- Native local message payload pass-through
- Exact type+version routing with optional fallback
- Rejection outcomes for unsupported type, nil payload, and version mismatch
- Observable routing and rejection events

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/benchmark -run '^$' -bench BenchmarkLocalMessagingPerformance -benchmem -benchtime=3s
```

## 4. Validate Performance Gates

```bash
WC_BENCH_TARGET=50000 go test ./tests/benchmark -run '^$' -bench BenchmarkLocalMessagingPerformance -benchmem -benchtime=1s
WC_ENFORCE_LOCAL_PERF_GATE=1 WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$' -bench BenchmarkLocalMessagingPerformance -benchmem -benchtime=3s
```

Pass criteria:
- p95 local send latency <= 25 µs
- Throughput >= 1,000,000 msgs/sec

## 5. Constitution Alignment Checks
- Local path remains Go-native with no mandatory schema serializers.
- Failure outcomes are explicit and supervision-compatible.
- Caller-facing contracts remain location-transparent.
- Performance and observability gates are measured and reported.

## 6. Expected Outcomes
- Local native structured messages are delivered without local serialization.
- Type routing is deterministic and testable.
- Rejection outcomes are explicit and observable.
- Performance thresholds are met on baseline profile.
