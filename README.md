# westcoast

Go actor runtime inspired by Erlang/OTP. Single-node execution with built-in distribution support for multi-node clusters.

Zero external dependencies. Single Go module.

## What It Provides

### Actor Runtime (`src/actor`)

- **Actor lifecycle**: create, run, stop, restart via supervision policies.
- **Mailbox-based messaging**: bounded async FIFO with backpressure (configurable capacity, explicit rejection on full).
- **PID addressing**: location-transparent PIDs with namespace/generation. Stale PID references are rejected.
- **Ask/Reply**: request-response with timeout. Supports asynchronous delegation (capture `replyTo`, reply later from any goroutine).
- **Name registry**: register/lookup/unregister actor names with lifecycle cleanup on stop.
- **Type routing**: content-based pre-delivery filtering by `TypeName` + `SchemaVersion` (exact match, fallback, version-mismatch rejection).
- **Router pools**: `round_robin`, `random`, and `consistent_hash` dispatch to worker actors.
- **Mailbox batching**: opt-in bounded batch dequeue with `BatchReceive` handler.
- **Supervision**: pluggable `SupervisorPolicy` interface. `DefaultSupervisor` with configurable restart limit. Mailbox preserved across restarts.
- **Lifecycle hooks**: `Start` and `Stop` hooks with configurable timeouts. Panic-safe (defer/recover).
- **PubSub broker**: built-in broker actor with MQTT-style wildcard routing (`+` single-segment, `#` tail wildcard). Ask-based subscribe/unsubscribe. Async publish fan-out.
- **Observability**: event emitter interface, outcome journal (processing, lifecycle, ask, routing, batch, broker outcomes), pluggable metrics hooks.
- **Guardrails**: PID-only cross-actor policy mode, gateway route modes, distributed readiness validation.

### Distributed Cluster (`src/actor/cluster`)

- **Cluster formation**: pluggable `ClusterProvider` for node discovery. `FixedProvider` (static seed list with heartbeat failure detection) included. Multicast, K8s, Consul providers can be added.
- **Transport**: pluggable `Transport` interface. TCP transport with length-prefixed gob frames, handshake-based authentication. gRPC or QUIC transports can be substituted.
- **Authentication**: pluggable `ClusterAuth`. `SharedSecretAuth` (constant-time comparison) and `NoopAuth` included.
- **Codec**: pluggable `Codec` for message serialization. `GobCodec` included.
- **Ring topology**: consistent hash ring with configurable fanout and finger tables. O(log n) routing with bounded hop count. Connection count scales O(log n) vs O(n) for full mesh. `FullMeshTopology` also available for small clusters.
- **Remote messaging**: transparent cross-node PID sends. `sendPIDWithSender` detects remote PIDs and routes through the transport layer. Ask/Reply works across nodes (node-qualified `__ask_reply@nodeID` namespaces).
- **Distributed registry** (two strategies):
  - `CRDTRegistry`: eventually consistent, backed by `crdt.ORSet`. Digest-based anti-entropy gossip with tombstone compaction. Add-wins OR-Set semantics with LWW conflict resolution and deterministic tiebreak.
  - `PartitionedRegistry`: hash-ring sharded. Deterministic name ownership via `Topology.Responsible`. Strong consistency for name lookups. Rebalances on membership changes.
- **Distributed PubSub** (Phoenix.PubSub model): subscriptions stay local, publications broadcast to all nodes. `DirectPubSubAdapter` (fan-out to all) and `GossipPubSubAdapter` (epidemic gossip) included. No re-broadcast loops.
- **Leader election**: `RingElection` — deterministic from membership (hash-based, no voting protocol). Scoped elections (multiple independent elections concurrently). Monotonically increasing terms for fencing.
- **Cluster supervision**: user-space `ClusterSupervisor` that watches for node failures, uses leader election for single-decision-maker semantics, and delegates to a pluggable `ClusterSupervisionPolicy` for placement decisions. `SimpleRestartPolicy` included.
- **Membership events**: `cluster.membership` PubSub topic for actors to observe join/leave/fail. `Runtime.ClusterMembers()` query API.
- **Gossip protocol**: generic `GossipProtocol` with configurable interval and fanout. Used by CRDT registry and gossip PubSub adapter.

### CRDT Library (`src/crdt`)

Standalone, extractable CRDT package with zero dependencies on the actor system.

- `VectorClock` / `VectorTimestamp`: causal ordering, merge, happens-before, concurrency detection.
- `Tag`: unique operation identifier with `Dominates()` for deterministic tiebreak.
- `ORSet`: Observed-Remove Set with digest-based anti-entropy. `Add`, `Put`, `Remove`, `RemoveIf`, `Filter`. `Digest` / `DeltaFor` / `MergeDelta` for synchronization. Tombstone compaction with configurable TTL. Thread-safe.

## Repository Layout

```text
src/
  actor/                  # Runtime, actor refs, mailbox, PID resolver, supervision,
                          # events, outcomes, pubsub broker, type routing
  actor/cluster/          # Cluster formation, transport, topology, remote messaging,
                          # distributed registry, distributed pubsub, leader election,
                          # cluster supervision, gossip protocol
  crdt/                   # Standalone CRDT library (OR-Set, vector clocks)
  internal/metrics/       # Metrics hook interface + no-op implementation

tests/
  unit/                   # Fast correctness tests
  integration/            # End-to-end runtime behavior tests
  contract/               # Caller-visible behavioral contracts
  benchmark/              # Throughput/latency benchmark suites

specs/                    # Feature specs/plans/tasks/checklists
```

## Requirements

- Go 1.24+ (see `go.mod`)
- macOS/Linux
- No external dependencies

## Build and Test

```bash
# all tests (actor + cluster + crdt)
go test ./src/...

# race detector
go test -race ./src/...

# static checks
go vet ./src/...
```

## Quick Start — Single Node

```go
rt := actor.NewRuntime()

ref, _ := rt.CreateActor("counter", 0, func(_ context.Context, state any, msg actor.Message) (any, error) {
    return state.(int) + msg.Payload.(int), nil
})

ref.Send(context.Background(), 3)

pid, _ := ref.PID("default")
ref.SendPID(context.Background(), pid, 2)
```

## Quick Start — Two-Node Cluster

```go
// Node 1
codec := cluster.NewGobCodec()
transport1 := cluster.NewGRPCTransport("node-1")
provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
    Seeds: []string{"10.0.0.2:9000"},
})

c1, _ := cluster.NewCluster(cluster.ClusterConfig{
    Self:      cluster.NodeMeta{ID: "node-1", Addr: "10.0.0.1:9000"},
    Provider:  provider1,
    Transport: transport1,
    Codec:     codec,
})

remoteSender := cluster.NewRemoteSender(c1, codec, metrics.NopHooks{})

rt1 := actor.NewRuntime(
    actor.WithNodeID("node-1"),
    actor.WithRemoteSend(remoteSender.Send),
)

// Wire inbound delivery
dispatcher := cluster.NewInboundDispatcher(rt1, codec)
c1.cfg.OnEnvelope = func(from cluster.NodeID, env cluster.Envelope) {
    dispatcher.Dispatch(context.Background(), env)
}

c1.Start(context.Background())

// Now rt1.SendPID(ctx, remotePID, payload) transparently routes
// to actors on node-2 via the transport layer.
```

## Key Runtime APIs

### Runtime Creation

- `NewRuntime(opts...)` — creates a runtime. Options:
  - `WithSupervisor(policy)`, `WithEmitter(e)`, `WithMetrics(h)`
  - `WithNodeID(id)` — sets the local node identity for cluster operation
  - `WithRemoteSend(fn)`, `WithRemoteAskSend(fn)` — injects remote transport
  - `WithPubSubBroadcast(fn)` — injects cross-node pubsub broadcast
  - `WithClusterRegistry(register, lookup, unregister)` — injects distributed registry
  - `WithClusterMembers(fn)` — injects membership query

### Actor Lifecycle

- `CreateActor(id, initialState, handler, opts...)` — options:
  - `WithMailboxCapacity(n)`, `WithStartHook(h)`, `WithStopHook(h)`
  - `WithStopHookTimeout(d)`, `WithBatching(maxSize, receiver)`
- `ActorRef.Stop()`, `ActorRef.Status()`

### Messaging

- `ActorRef.Send(ctx, payload)` — fire-and-forget
- `ActorRef.Ask(ctx, payload, timeout)` — request-response
- `ActorRef.SendPID(ctx, pid, payload)` — PID-addressed (local or remote)
- `ActorRef.CrossSendActorID(ctx, targetID, payload)` — cross-actor by ID
- `ActorRef.CrossSendPID(ctx, pid, payload)` — cross-actor by PID

### PID and Discovery

- `IssuePID(namespace, actorID)` — namespace defaults to node ID when clustered
- `ResolvePID(pid)`, `PIDForActor(actorID)`
- `RegisterName(actorID, name, namespace)`, `LookupName(name)`, `UnregisterName(name)`
- `LookupName` falls back to cluster registry when local miss

### PubSub

- `EnsureBrokerActor(brokerID)` — creates or reuses the broker actor
- `BrokerSubscribe(ctx, brokerID, pid, pattern, timeout)`
- `BrokerUnsubscribe(ctx, brokerID, pid, pattern, timeout)`
- `BrokerPublish(ctx, brokerID, topic, payload, publisherActorID)`
- `BrokerPublishRemote(ctx, brokerID, topic, payload, publisherActorID)` — injects remote publications without re-broadcast

### Cluster

- `NodeID()` — local node identity
- `ClusterMembers()` — current membership
- `PublishMembershipEvent(ctx, event)` — emits on `cluster.membership` topic

### Observability

- `Outcome(messageID)`, `LifecycleOutcomes(actorID)`
- `GuardrailOutcomes(actorID)`, `AskOutcomes(actorID)`
- `RoutingOutcomes(routerID)`, `BatchOutcomes(actorID)`
- `BrokerOutcomes(brokerID)`, `BrokerPublishedCount(brokerID)`

## Architecture

```text
┌─────────────────────────────────────────────┐
│              User Code / Actors              │
├─────────────────────────────────────────────┤
│          Runtime (actor lifecycle)           │
│   Local mailbox delivery, supervision,      │
│   ask/reply, type routing, batching         │
├──────────┬──────────┬───────────────────────┤
│ Registry │  PubSub  │   PIDResolver         │
│ Strategy │  Adapter │   (local + remote)    │
├──────────┴──────────┴───────────────────────┤
│       Cluster Topology (ring / full mesh)   │
│   Finger tables, multi-hop forwarding       │
├─────────────────────────────────────────────┤
│       ClusterProvider (Fixed / custom)      │
│   Membership events, failure detection      │
├─────────────┬───────────────────────────────┤
│   Codec     │   Transport (TCP / custom)    │
│  (gob)      │   + ClusterAuth               │
└─────────────┴───────────────────────────────┘
```

## Pluggable Interfaces

Every distributed concern is behind an interface. Defaults work out of the box; swap implementations for your environment.

| Interface | Default | Purpose |
|---|---|---|
| `ClusterProvider` | `FixedProvider` | Node discovery and failure detection |
| `Transport` | TCP with gob frames | Inter-node communication |
| `ClusterAuth` | `NoopAuth` | Connection authentication |
| `Codec` | `GobCodec` | Message serialization |
| `Topology` | `FullMeshTopology` | Connection decisions and message routing |
| `RegistryStrategy` | (interface) | Distributed name registry |
| `PubSubAdapter` | `DirectPubSubAdapter` | Cross-node publication broadcast |
| `LeaderElection` | `RingElection` | Scoped leader election |
| `ClusterSupervisionPolicy` | `SimpleRestartPolicy` | Actor placement on node failure |
| `SupervisorPolicy` | `DefaultSupervisor` | Local actor restart decisions |

## Feature Specs

Documented under `specs/`:

- `001-actor-execution-engine`
- `002-location-transparent-pids`
- `003-local-struct-messaging`
- `004-supervisor-fault-tolerance`
- `005-actor-registry-discovery`
- `006-lifecycle-hooks`
- `007-pid-gateway-guardrails`
- `008-ask-reply-semantics`
- `009-actor-router-pools`
- `010-mailbox-batching`
- `011-native-pubsub-broker`
