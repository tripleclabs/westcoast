# westcoast

Go-native actor runtime focused on deterministic single-node execution with explicit seams for future distributed routing.

## What It Provides

- Actor lifecycle: create, run, stop, restart via supervision policies.
- Asynchronous mailbox-based message processing.
- Type-based local routing (`exact` + `fallback`) with deterministic rejections.
- Location-transparent PID addressing and generation-safe PID delivery.
- Name registry/discovery (`register`, `lookup`, `unregister`) with lifecycle cleanup.
- Lifecycle hooks (`Start`, `Stop`) with observable outcomes.
- Router and worker-pool dispatch:
  - Stateless strategies (`round_robin`, `random`).
  - Consistent-hash sharding for `HashKey()` messages.
  - Deterministic routing outcomes (`route_success`, `route_failed_*`).
- Smart mailbox batching:
  - Opt-in bounded batch retrieval per actor cycle.
  - `BatchReceive([]any)`-style handling for grouped payload processing.
  - Batch lifecycle outcomes (`batch_success`, `batch_failed_*`).
- Distributed-readiness guardrails:
  - PID-only cross-actor policy mode.
  - Pluggable gateway route modes (`local_direct`, `gateway_mediated`).
  - Readiness validation outcomes (`pid_policy`, `gateway_boundary`, `location_transparency`).
- In-memory telemetry surfaces:
  - Event stream (`MemoryEmitter`).
  - Runtime metrics hooks (`internal/metrics`).

## Repository Layout

```text
src/
  actor/                  # Runtime, actor refs, mailbox, PID resolver, supervision, events, outcomes
  internal/metrics/       # Metrics interfaces + in-memory implementation

tests/
  unit/                   # Fast correctness tests
  integration/            # End-to-end runtime behavior tests
  contract/               # Caller-visible behavioral contracts
  benchmark/              # Throughput/latency benchmark suites

specs/                    # Feature specs/plans/tasks/checklists
```

## Requirements

- Go `1.24+` (see `go.mod`)
- macOS/Linux

## Build, Test, Lint

```bash
# format
gofmt -w ./src ./tests

# static checks
go vet ./...

# all tests
go test ./...

# race checks (recommended)
go test -race ./tests/unit/... ./tests/integration/... ./tests/contract/...
```

Using `Makefile` shortcuts:

```bash
make fmt
make lint
make test
```

## Benchmarks

```bash
# PID resolver latency
make bench-pid

# local messaging throughput/latency
make bench-local-messaging
```

Optional environment variables used by benchmark suites:

- `WC_BENCH_TARGET`
- `WC_BENCH_PROFILE`
- `WC_LAT_SAMPLE_EVERY`
- `WC_LAT_MAX_SAMPLES`
- `WC_ENFORCE_LOCAL_PERF_GATE`

## Quick Start

```go
package main

import (
    "context"
    "fmt"

    "westcoast/src/actor"
)

func main() {
    rt := actor.NewRuntime()

    ref, err := rt.CreateActor("counter", 0, func(_ context.Context, state any, msg actor.Message) (any, error) {
        n := state.(int)
        switch msg.Payload.(type) {
        case int:
            return n + msg.Payload.(int), nil
        default:
            return n, nil
        }
    })
    if err != nil {
        panic(err)
    }

    ack := ref.Send(context.Background(), 3)
    fmt.Println("send result:", ack.Result)

    pid, _ := ref.PID("default")
    pAck := ref.SendPID(context.Background(), pid, 2)
    fmt.Println("pid outcome:", pAck.Outcome)
}
```

## Key Runtime APIs

From [`src/actor/runtime.go`](/Volumes/Store1/src/3clabs/westcoast/src/actor/runtime.go) and [`src/actor/actor_ref.go`](/Volumes/Store1/src/3clabs/westcoast/src/actor/actor_ref.go):

- Runtime creation/options:
  - `NewRuntime(...)`
  - `WithSupervisor(...)`, `WithEmitter(...)`, `WithMetrics(...)`
- Actor creation/options:
  - `CreateActor(id, initialState, handler, opts...)`
  - `WithMailboxCapacity(n)`
  - `WithStartHook(h)`, `WithStopHook(h)`
  - `WithStopHookTimeout(d)`
  - `WithBatching(maxBatchSize, receiver)`
- Messaging:
  - `ActorRef.Send(ctx, payload)`
  - `ActorRef.Ask(ctx, payload, timeout)`
  - `ActorRef.SendPID(ctx, pid, payload)`
  - `ActorRef.CrossSendActorID(ctx, targetActorID, payload)`
  - `ActorRef.CrossSendPID(ctx, pid, payload)`
- PID/discovery:
  - `IssuePID`, `ResolvePID`, `PIDForActor`
  - `RegisterName`, `LookupName`, `UnregisterName`
- Guardrails:
  - `SetPIDInteractionPolicy(...)`
  - `SetGatewayRouteMode(...)`
  - `SetGatewayAvailable(bool)`
  - `ValidateDistributedReadiness()`
- Observability access:
  - `Outcome(messageID)`
  - `LifecycleOutcomes(actorID)`
  - `GuardrailOutcomes(actorID)`
  - `AskOutcomes(actorID)`
  - `RoutingOutcomes(routerID)`
  - `BatchOutcomes(actorID)`
- Router surfaces:
  - `ActorRef.ConfigureRouter(strategy, workers)`
  - `ActorRef.Route(ctx, payload)`
- Batching surfaces:
  - `ActorRef.ConfigureBatching(maxBatchSize, receiver)`
  - `ActorRef.DisableBatching()`

## Ask Request-Response Pattern

Use `Ask` when you need request-response semantics with timeout-bounded waiting.

```go
res, err := ref.Ask(context.Background(), askRequest{Value: 1}, 500*time.Millisecond)
if err != nil {
    // timeout/canceled/invalid target
}
reply := res.Payload
```

Ask-originated messages include implicit context:
- `msg.IsAsk()`
- `msg.AskRequestID()`
- `msg.AskReplyTo()`

This enables asynchronous delegation:
1. Receive Ask message in handler.
2. Capture `replyTo` + `requestID`.
3. Delegate long work in a goroutine.
4. Reply later via `SendPID(replyTo, ...)`.

## Feature Specs

Current feature sets are documented under `specs/`:

- `001-actor-execution-engine`
- `002-location-transparent-pids`
- `003-local-struct-messaging`
- `004-supervisor-fault-tolerance`
- `005-actor-registry-discovery`
- `006-lifecycle-hooks`
- `007-pid-gateway-guardrails`
- `008-ask-request-response`
- `009-actor-router-pools`
- `010-mailbox-batching`

Each spec folder includes `spec.md`, `plan.md`, `tasks.md`, and supporting contract/data/research docs.

## Operational Notes

- Runtime is in-memory and process-local.
- No persistence or multi-node transport implementation is included yet.
- Stop hooks are executed with a bounded timeout and force terminal actor stop on deadline.
- Cross-actor message guardrails are policy-driven so PID-only routing can be enforced before multi-node transport is introduced.
- For production use, review [PRODUCTION_READINESS_AUDIT.md](/Volumes/Store1/src/3clabs/westcoast/PRODUCTION_READINESS_AUDIT.md).

## CI and Release Management

- CI workflow: [`.github/workflows/ci.yml`](/Volumes/Store1/src/3clabs/westcoast/.github/workflows/ci.yml)
  - Enforces `gofmt`, `go vet`, full tests, race suite, and benchmark smoke checks.
- Release policy: [`RELEASE.md`](/Volumes/Store1/src/3clabs/westcoast/RELEASE.md)
- Changelog policy + entries: [`CHANGELOG.md`](/Volumes/Store1/src/3clabs/westcoast/CHANGELOG.md)
