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
    dispatcher.Dispatch(context.Background(), from, env)
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

## Interface Reference

All types below live in `src/actor/cluster/` unless noted as `actor.`.

### Core Types

```go
type NodeID string

type NodeMeta struct {
    ID       NodeID
    Addr     string            // host:port for transport connections
    Tags     map[string]string // arbitrary metadata (region, zone, capabilities)
    JoinedAt time.Time
}

type MemberEventType int // MemberJoin, MemberLeave, MemberFailed, MemberUpdated

type MemberEvent struct {
    Type   MemberEventType
    Member NodeMeta
}
```

### Envelope (wire format)

```go
type Envelope struct {
    SenderNode    NodeID
    SenderActorID string
    TargetNode    NodeID
    TargetActorID string
    Namespace     string // PID namespace
    Generation    uint64 // PID generation
    TypeName      string
    SchemaVersion string
    MessageID     uint64
    Payload       []byte // codec-encoded
    IsAsk         bool
    AskRequestID  string
    AskReplyTo    *RemotePID // nil for fire-and-forget
    SentAtUnixNano int64
}

type RemotePID struct {
    Node       NodeID
    Namespace  string
    ActorID    string
    Generation uint64
}
```

### Transport

```go
type Transport interface {
    Listen(addr string, handler InboundHandler) error
    Dial(ctx context.Context, addr string, auth ClusterAuth) (Connection, error)
    Close() error
}

type Connection interface {
    Send(ctx context.Context, env Envelope) error
    Close() error
    RemoteAddr() string
    RemoteNodeID() NodeID
}

type InboundHandler interface {
    OnEnvelope(from NodeID, env Envelope)
    OnConnectionEstablished(remote NodeID, conn Connection)
    OnConnectionLost(remote NodeID, err error)
}
```

### ClusterProvider

```go
type ClusterProvider interface {
    Start(self NodeMeta) error
    Stop() error
    Members() []NodeMeta
    Events() <-chan MemberEvent
}
```

### ClusterAuth

```go
type ClusterAuth interface {
    Credentials() ([]byte, error)
    Verify(peerCredentials []byte) error
}
```

### Codec

```go
type Codec interface {
    Encode(v any) ([]byte, error)
    Decode(data []byte, v any) error
    Register(v any) // make a concrete type known (gob.Register)
    Name() string   // "gob", "proto", etc.
}
```

### Topology

```go
type Topology interface {
    ShouldConnect(self NodeID, members []NodeMeta) []NodeID
    Route(self, target NodeID, members []NodeMeta) (nextHop NodeID, ok bool)
    Responsible(key string, members []NodeMeta, replication int) []NodeID
}
```

### LeaderElection

```go
type LeaderElection interface {
    Leader(scope string) (NodeID, bool)
    IsLeader(scope string) bool
    Term(scope string) uint64
    Watch(scope string) <-chan LeaderEvent
    OnMembershipChange(event MemberEvent)
}

type LeaderEvent struct {
    Scope      string
    Leader     NodeID
    PrevLeader NodeID
    Term       uint64
}
```

### ClusterConfig

```go
type ClusterConfig struct {
    Self      NodeMeta
    Provider  ClusterProvider
    Transport Transport
    Auth      ClusterAuth       // defaults to NoopAuth
    Codec     Codec             // defaults to GobCodec
    Topology  Topology          // defaults to FullMeshTopology

    // Callbacks — set by the integration layer before Start.
    OnEnvelope    func(from NodeID, env Envelope)
    OnMemberEvent func(event MemberEvent)
}
```

### Runtime Cluster Wiring (package `actor`)

Function types injected via `RuntimeOption` to connect the Runtime to the cluster layer without import cycles:

```go
// Injected via WithRemoteSend — called when SendPID targets a remote node.
type RemoteSenderFunc func(
    ctx context.Context, senderActorID string, pid PID,
    payload any, msgID uint64,
) (PIDSendAck, error)

// Injected via WithRemoteAskSend — called for cross-node Ask requests.
type RemoteAskSenderFunc func(
    ctx context.Context, senderActorID string, pid PID,
    payload any, msgID uint64,
    askRequestID string, replyTo PID,
) (PIDSendAck, error)

// Injected via WithPubSubBroadcast — called after local publish to fan out.
type PubSubBroadcastFunc func(ctx context.Context, topic string, payload any) error
```

### RegistryStrategy

```go
type RegistryStrategy interface {
    Register(name string, pid actor.PID) error
    Lookup(name string) (actor.PID, bool)
    Unregister(name string) (actor.PID, bool)
    UnregisterByNode(node NodeID) []string
    OnMembershipChange(event MemberEvent)
}
```

### PubSubAdapter

```go
type PubSubAdapter interface {
    Broadcast(ctx context.Context, topic string, payload any, publisherNode NodeID) error
    SetHandler(handler RemotePublishHandler)
    Start(ctx context.Context) error
    Stop() error
}

type RemotePublishHandler func(topic string, payload any, publisherNode NodeID)
```

### ClusterSupervisionPolicy

```go
type ClusterSupervisionPolicy interface {
    OnNodeFailed(failedNode NodeID, actorNames []string, liveMembers []NodeMeta) []PlacementDecision
}

type PlacementDecision struct {
    ActorName  string
    TargetNode NodeID
    Action     PlacementAction // PlacementRestart or PlacementAbandon
}
```

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
