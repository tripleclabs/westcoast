# Quickstart: Validate Actor Routers and Worker Pools

## Prerequisites
- Go 1.24+
- Repository root: `/Volumes/Store1/src/3clabs/westcoast`

## 1. Validate Router Test Coverage

```bash
go test ./tests/integration/... -run TestRouterRoundRobinDistributesAcrossWorkers
go test ./tests/integration/... -run TestRouterRandomStrategyDistributesAcrossWorkers
go test ./tests/integration/... -run TestConsistentHashRoutesSameKeyToSameWorker
go test ./tests/integration/... -run TestRouterRejectsConsistentHashMessageWithoutKey
go test ./tests/contract/... -run TestRouterWorkerPoolContract
```

Expected on `009-actor-router-pools`: tests pass.  
Expected on a clean pre-feature branch: tests fail before implementation changes.

## 2. Implement/Review Minimum Feature Slice

Implement:
- Router dispatch surface for worker pool routing
- Round-robin and random stateless routing strategies
- Consistent-hash key-based worker sharding
- Deterministic failure outcomes for no workers, invalid keys, and unavailable workers
- Routing observability outcomes

## 3. Re-run Verification

```bash
go test ./tests/unit/...
go test ./tests/integration/...
go test ./tests/contract/...
```

## 4. Validate Critical Behavior

```bash
go test ./tests/integration/... -run 'TestRouterRoundRobinDistributesAcrossWorkers|TestRouterRandomStrategyDistributesAcrossWorkers|TestConsistentHashRoutesSameKeyToSameWorker|TestRouterWorkerFailureProducesDeterministicOutcome'
```

Pass criteria:
- Round-robin strategy distributes work predictably.
- Random strategy distributes across the configured pool.
- Same hash key always routes to same worker for stable pool.
- Router failure outcomes remain explicit and deterministic.

## 5. Constitution Alignment Checks
- Runtime implementation remains Go-native and lightweight.
- Router/worker faults remain explicit and supervision-compatible.
- Failing-then-passing tests prove routing behavior changes.
- Router-facing contracts remain location-transparent.
- Strategy logic remains simple with measurable overhead.

## 6. Expected Outcomes
- Senders can parallelize workload through router actors without direct worker management.
- Stateful keyed workloads avoid state-locking conflicts via deterministic shard routing.
- Routing contracts are ready for future distributed transport without caller API rewrites.
