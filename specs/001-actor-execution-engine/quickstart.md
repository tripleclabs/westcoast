# Quickstart: Validate Actor Execution Engine

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Run Tests First (Red)

```bash
go test ./tests/unit/... -run TestActorStateIsolation
go test ./tests/unit/... -run TestMailboxOrdering
go test ./tests/integration/... -run TestActorFailureIsolation
```

Expected: new tests fail before implementation.

## 2. Implement Minimum Runtime Behavior (Green)

Implement the minimum feature slice needed to pass:
- Actor create/start/stop lifecycle
- Per-actor mailbox enqueue + sequential dequeue processing
- Non-blocking submit outcomes for full/stopped/not found actors
- Outcome/event emission for success/failure paths

## 3. Re-run Full Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/benchmark -run '^$' -bench BenchmarkMillionActors -benchmem -benchtime=1x
```

## 4. Validate Constitution Gates
- Go-native runtime only (no external schema-first serialization in hot path)
- Failure isolation behavior verified by integration test
- Logical actor ID contracts verified by contract tests
- Performance target checks captured in benchmark output

## 5. Expected Outcomes
- State isolation tests pass with no direct external mutation path.
- Message order tests pass with FIFO per actor.
- Failure isolation tests show unaffected actors remain active after injected fault.
- Benchmark run validates target actor scale profile.

## 6. Benchmark Profile (200k Actors) and Machine Capture

Use this profile for repeatable performance checks before moving to full-scale runs.

```bash
# Capture environment
go version
uname -a

# Serial send-path benchmark at 200k actors
WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$' -bench BenchmarkMillionActors$ -benchmem -benchtime=3s

# Parallel send-path benchmark at 200k actors
WC_BENCH_TARGET=200000 go test ./tests/benchmark -run '^$' -bench BenchmarkMillionActorsParallelSend$ -benchmem -benchtime=3s

# Parallel enqueue latency benchmark with percentile reporting
WC_BENCH_TARGET=200000 WC_LAT_SAMPLE_EVERY=64 WC_LAT_MAX_SAMPLES=200000 \
go test ./tests/benchmark -run '^$' -bench BenchmarkMillionActorsParallelLatency$ -benchmem -benchtime=3s
```

Record these fields for each run:
- `goos`, `goarch`, `cpu`
- `ns/op`, `B/op`, `allocs/op`
- `p50_enqueue_ns`, `p95_enqueue_ns`, `p99_enqueue_ns` (latency benchmark)
- `WC_BENCH_TARGET`, `WC_LAT_SAMPLE_EVERY`, `WC_LAT_MAX_SAMPLES`, `-benchtime`
