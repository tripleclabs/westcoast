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
- **Timers**: `SendAfter(pid, payload, delay)` and `SendInterval(pid, payload, interval)`. Returns `TimerRef` for cancellation. Works with both local and remote PIDs.
- **Actor monitors**: `Monitor(watcher, target)` — watcher receives a `DownMessage` when the target stops (for any reason: explicit stop, supervision decision, crash). `Demonitor` to cancel. Works cross-node.
- **Dead letters**: undeliverable messages routed to a configurable `DeadLetterHandler`. Every PID rejection (not found, stopped, stale generation) emits a `DeadLetter` with the original payload, target, and reason. Counter available via `DeadLetterCount()`.
- **Observability**: event emitter interface, outcome journal (processing, lifecycle, ask, routing, batch, broker outcomes), pluggable metrics hooks.
- **Guardrails**: PID-only cross-actor policy mode, gateway route modes, distributed readiness validation.

### Distributed Cluster (`src/actor/cluster`)

- **Cluster formation**: pluggable `ClusterProvider` for node discovery. `FixedProvider` (static seed list with heartbeat failure detection) built-in. Optional provider modules in `pkg/`: `multicastprovider` (UDP LAN discovery), `awsprovider` (EC2 tags/ASG), `gcpprovider` (GCE instances), `azureprovider` (Azure VMs). Each is a separate Go module — `go get` only pulls the SDK you need.
- **Transport**: pluggable `Transport` interface. `TCPTransport` with length-prefixed gob frames, handshake-based authentication, optional TLS encryption (via `TCPTransportConfig`). `grpctransport.GRPCTransport` with protobuf envelopes, bidirectional streaming, gateway-friendly routing headers, and gob-encoded payloads. QUIC transports can be substituted.
- **Authentication**: pluggable `ClusterAuth`. `SharedSecretAuth` (constant-time comparison), `CertAuth` (x509 certificate verification against a cluster CA), and `NoopAuth` included.
- **Codec**: pluggable `Codec` for message serialization. `GobCodec` included.
- **Ring topology**: consistent hash ring with configurable fanout and finger tables. O(log n) routing with bounded hop count. Connection count scales O(log n) vs O(n) for full mesh. `FullMeshTopology` also available for small clusters.
- **Remote messaging**: transparent cross-node PID sends. `sendPIDWithSender` detects remote PIDs and routes through the transport layer. Ask/Reply works across nodes (node-qualified `__ask_reply@nodeID` namespaces).
- **Distributed registry**: `DistributedRegistry` backed by `crdt.ORMap` from `github.com/tripleclabs/crdt-go`. Add-wins semantics, automatic delta replication and anti-entropy via the CRDT library's transport layer. No hand-rolled gossip. `NewDistributedRegistry(nodeID, WithClusterReplication(cluster))` wires up CRDT transport automatically.
- **Distributed PubSub** (Phoenix.PubSub model): subscriptions stay local, publications broadcast to other nodes. Three adapters: `DirectPubSubAdapter` (fan-out to all nodes), `GossipPubSubAdapter` (batched gossip), and `CRDTPubSubAdapter` (CRDT-replicated routing table — sends only to nodes with subscribers for the topic, major bandwidth reduction in large clusters).
- **Leader election**: `RingElection` — deterministic from membership (hash-based, no voting protocol). Scoped elections (multiple independent elections concurrently). Monotonically increasing terms for fencing.
- **Cluster supervision**: user-space `ClusterSupervisor` that watches for node failures, uses leader election for single-decision-maker semantics, and delegates to a pluggable `ClusterSupervisionPolicy` for placement decisions. `SimpleRestartPolicy` included.
- **Cluster router**: `ClusterRouter` provides distributed service groups with metadata-aware routing. Workers on any node join a named service. Supports static filtering (`WithWorkerFilter`), static preference (`WithWorkerPreference`), and call-time locality-aware routing (`SendWith` + `Nearest`). Strategies: round-robin, random, consistent-hash. Worker lists replicate via CRDT gossip. No coordinator node.
- **Singleton actors**: `SingletonManager` guarantees exactly one instance of an actor runs cluster-wide. Each singleton gets its own election scope — singletons distribute across nodes via consistent hashing (not all on the leader). On leadership change, a **handoff protocol** coordinates the transition: the new leader requests the old leader to stop the singleton and optionally transfer state before starting it. This guarantees at-most-one semantics — the new instance never starts until the old one confirms it has stopped. Configurable `HandoffTimeout` with automatic fallback if the old leader is unreachable. Cluster-of-one incurs zero handoff overhead. Optional `Placement` predicates restrict which nodes can host a singleton.
- **DaemonSet actors**: `DaemonSetManager` runs an actor on every node. Cross-node addressing via `SendTo(ctx, name, nodeID, payload)` and `AskTo` — no PID construction, no generation guessing. `Broadcast` sends to all nodes. Works seamlessly in single-node mode. Supports **replicated state** via `RegisterReplicated[V]` — each instance gets a `crdt.ORMap[V]` with add-wins semantics that converges automatically across the cluster. Reads are local, writes replicate via the CRDT library's transport and anti-entropy.
- **Distributed Ask**: `AskPID(ctx, pid, payload, timeout)` — request-response across nodes. The reply traverses the transport back via node-qualified `__ask_reply@nodeID` namespaces.
- **Graceful drain**: `Drain(ctx, cluster, cfg, opts...)` — planned shutdown. Emits `MemberLeave` (vs `MemberFailed`), waits a configurable handoff grace period for singleton handoff requests to complete, stops singletons and daemons, deregisters names, waits for in-flight work, then stops transport.
- **Dynamic node metadata**: `UpdateTags(map[string]string)` sets runtime metadata on a node (region, GPU count, rack, etc.). Tags gossip to all peers automatically. Changed tags emit `MemberUpdated` events. Queryable via `Members()` and `Self()`. Providers (e.g. AWS) contribute infrastructure tags at start; applications add their own at runtime.
- **Membership events**: `cluster.membership` PubSub topic for actors to observe join/leave/fail/update. `Runtime.ClusterMembers()` query API.
- **Gossip protocol**: generic `GossipProtocol` with `GossipRouter` for multiplexing. Used by CRDT registry, PubSub adapter, and metadata gossip.

### CRDT Library (`src/crdt`)

Standalone, extractable CRDT package with zero dependencies on the actor system.

- `VectorTimestamp`: causal ordering, merge, happens-before, concurrency detection.
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
transport1 := cluster.NewTCPTransport("node-1")
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
  - `WithSupervisor(policy)`, `WithEmitter(e)`, `WithMetrics(h)`, `WithDeadLetterHandler(fn)`
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
- `RemoveActor(id)` — purge a stopped actor from the registry so its ID can be reused by `CreateActor`

### Messaging

- `ActorRef.Send(ctx, payload)` — fire-and-forget
- `ActorRef.Ask(ctx, payload, timeout)` — request-response (local actor by ID)
- `AskPID(ctx, pid, payload, timeout)` — request-response by PID (local or remote)
- `ActorRef.SendPID(ctx, pid, payload)` — PID-addressed fire-and-forget (local or remote)
- `ActorRef.CrossSendActorID(ctx, targetID, payload)` — cross-actor by ID
- `ActorRef.CrossSendPID(ctx, pid, payload)` — cross-actor by PID

### PID and Discovery

- `IssuePID(namespace, actorID)` — namespace defaults to node ID when clustered
- `ResolvePID(pid)`, `PIDForActor(actorID)`
- `RegisterName(actorID, name, namespace)`, `LookupName(name)`, `UnregisterName(name)`
- `LookupName` falls back to cluster registry when local miss
- `SendName(ctx, name, payload)` — lookup + send in one call (local or remote)
- `AskName(ctx, name, payload, timeout)` — lookup + ask in one call (local or remote)

### Timers

- `SendAfter(target PID, payload, delay)` → `*TimerRef`
- `SendInterval(target PID, payload, interval)` → `*TimerRef`
- `TimerRef.Cancel()` — stops the timer
- `CancelTimer(ref)` — convenience alias

### Monitors

- `Monitor(watcher PID, target PID)` → `MonitorRef`
- `Demonitor(ref MonitorRef)` — cancel a monitor
- Target stops → watcher receives `DownMessage{Ref, Target, Reason}`

### Dead Letters

- `WithDeadLetterHandler(fn)` — set handler on runtime creation
- `DeadLetterCount()` — total undeliverable messages since startup
- `DeadLetter{TargetActorID, TargetPID, Payload, Reason, Timestamp}`

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
- `AskPID(ctx, pid, payload, timeout)` — distributed request-response

### Node Metadata

```go
// Set tags at runtime — gossiped to all peers automatically:
c.UpdateTags(map[string]string{
    "region":  "us-east-1",
    "gpus":    "4",
    "rack":    "rack-12",
})
c.RemoveTag("rack")

// Query local metadata:
self := c.Self()  // NodeMeta with current tags

// Query peer metadata (tags included):
for _, m := range c.Members() {
    fmt.Println(m.ID, m.Tags["region"], m.Tags["gpus"])
}
```

### Cluster Router (Distributed Services)

```go
cr := cluster.NewClusterRouter(rt, registry, c) // c = *Cluster, needed for metadata routing

// Basic routing:
cr.Configure("payment-processor", actor.RouterStrategyRoundRobin)
cr.Join("payment-processor", workerPID)
cr.Send(ctx, "payment-processor", payload)
cr.Ask(ctx, "payment-processor", payload, timeout)
cr.Broadcast(ctx, "payment-processor", payload)

// Metadata-aware routing (static):
cr.Configure("ml-inference", actor.RouterStrategyRoundRobin,
    cluster.WithWorkerFilter(cluster.TagGTE("gpus", 1)),        // only GPU nodes
    cluster.WithWorkerPreference(cluster.RankByTag("gpus", cluster.Highest)), // prefer most GPUs
)

// Locality-aware routing (call-time):
cr.SendWith(ctx, "api", payload,
    cluster.Nearest(map[string]string{
        "az":     "eu-west-1a",    // prefer same AZ (score 3 if all match)
        "region": "eu-west-1",     // then same region (score 2)
        "continent": "eu",         // then same continent (score 1)
    }),
)

// Static + call-time preferences combine — scores are summed.
```

### Singleton Actors

```go
// Single-node (no cluster):
sm := cluster.NewSingletonManager(rt, election, registry, nil, nil, nil)

// Clustered — handoff protocol ensures at-most-one during transitions:
sm := cluster.NewSingletonManager(rt, election, registry, c, codec, dispatcher)

// Wire membership events so the manager can skip handoff for crashed nodes:
c.SetOnMemberEvent(func(event cluster.MemberEvent) {
    sm.OnMemberEvent(event)
    election.OnMembershipChange(event)
})

sm.Register(cluster.SingletonSpec{
    Name:    "scheduler",
    Handler: schedulerHandler,
})

// With state handoff — old instance transfers state to new instance:
sm.Register(cluster.SingletonSpec{
    Name:           "rate-limiter",
    Handler:        rateLimiterHandler,
    HandoffTimeout: 15 * time.Second,
    CaptureState: func(ctx context.Context, actorID string) (any, error) {
        // Called on the OLD leader after stopping the actor.
        return currentRateLimits, nil
    },
    OnHandoff: func(state any) any {
        // Called on the NEW leader — returns initial state.
        return state // use transferred state as-is
    },
})

// Restrict to specific nodes:
sm.Register(cluster.SingletonSpec{
    Name:      "gpu-coordinator",
    Handler:   gpuHandler,
    Placement: cluster.TagGTE("gpus", 1),
})

sm.Start(ctx)
// Singletons distribute across nodes automatically.
// sm.Running() shows which ones are on this node.
```

### DaemonSet Actors

```go
dm := cluster.NewDaemonSetManager(rt, c, codec)
dm.Register(cluster.DaemonSpec{
    Name:    "coordinator",
    Handler: coordinatorHandler,
})
dm.Register(cluster.DaemonSpec{
    Name:    "metrics-collector",
    Handler: metricsHandler,
})
// Replicated daemon — shared CRDT state across all instances:
cluster.RegisterReplicated[Session](dm, "session-store",
    func(ctx context.Context, sessions *crdt.ORMap[Session], msg actor.Message) error {
        switch m := msg.Payload.(type) {
        case CreateSession:
            sessions.Put(ctx, m.ID, m.Session)
        case GetSession:
            s, _ := sessions.Get(m.ID)
            // reply with s...
        }
        return nil
    },
)

dm.Start(ctx)
// Daemons run on every node automatically.
// Replicated daemons converge state across all instances via CRDT.

// Send to a specific node's daemon — no PID needed:
dm.SendTo(ctx, "coordinator", "node-3", payload)

// Request-response to a specific node's daemon:
result, err := dm.AskTo(ctx, "coordinator", "node-3", request, 5*time.Second)

// Send to the daemon on ALL nodes:
dm.Broadcast(ctx, "metrics-collector", flushCommand)
```

### Graceful Drain

```go
// Planned shutdown (deploy, scale-down):
cluster.Drain(ctx, c, cluster.DrainConfig{Timeout: 30 * time.Second},
    cluster.WithSingletonManager(sm),   // singletons migrate to other nodes
    cluster.WithDaemonSetManager(dm),   // daemons stop
    cluster.WithRegistry(registry),     // names deregistered
    cluster.WithPubSubAdapter(adapter), // pubsub routing cleaned up
    cluster.WithHandoffGrace(3*time.Second), // wait for handoff requests
)
// Peers see MemberLeave (not MemberFailed), so no false failure recovery.
// During the handoff grace period, new singleton leaders can request state
// from this node before it shuts down.
```

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
| `ClusterAuth` | `NoopAuth` | Connection authentication (`SharedSecretAuth`, `CertAuth` also included) |
| `Codec` | `GobCodec` | Message serialization |
| `Topology` | `FullMeshTopology` | Connection decisions and message routing |
| `RegistryStrategy` | (interface) | Distributed name registry |
| `PubSubAdapter` | `DirectPubSubAdapter`, `GossipPubSubAdapter`, `CRDTPubSubAdapter` | Cross-node publication broadcast |
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

// TLS-enabled TCP transport:
type TCPTransportConfig struct {
    TLS *tls.Config // nil = plaintext (default)
}
func NewTCPTransportWithConfig(nodeID NodeID, cfg TCPTransportConfig) *TCPTransport

// gRPC transport (src/actor/cluster/grpctransport):
import "github.com/tripleclabs/westcoast/src/actor/cluster/grpctransport"

// Plaintext, no auth:
transport := grpctransport.New("node-1")

// HMAC shared-secret auth (replay-resistant via nonce+timestamp):
transport := grpctransport.New("node-1")
transport.SetAuth(cluster.NewSharedSecretAuth(secret))
conn, _ := transport.Dial(ctx, addr, cluster.NewSharedSecretAuth(secret))

// mTLS with per-message routing headers:
transport := grpctransport.NewWithConfig("node-1", grpctransport.Config{
    ServerTLS: &tls.Config{
        Certificates: []tls.Certificate{serverCert},
        ClientCAs:    caPool,
        ClientAuth:   tls.RequireAndVerifyClientCert,
    },
    ClientTLS: &tls.Config{
        Certificates: []tls.Certificate{clientCert},
        RootCAs:      caPool,
    },
    DefaultHeaders: map[string]string{
        grpctransport.HeaderTenantID: "tenant-42",
    },
})

// Per-message headers via HeaderConn:
conn, _ := transport.Dial(ctx, addr, auth)
conn.(grpctransport.HeaderConn).SendWithHeaders(ctx, env, map[string]string{
    grpctransport.HeaderTraceID:    traceID,
    grpctransport.HeaderRoutingKey: "shard-7",
})
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

Provider modules (separate `go get`):

```go
// LAN multicast discovery (dev / bare metal):
import "github.com/tripleclabs/westcoast/pkg/multicastprovider"
provider := multicastprovider.New()

// AWS EC2 — uses IAM role, discovers by tags:
import "github.com/tripleclabs/westcoast/pkg/awsprovider"
provider := awsprovider.NewWithConfig(awsprovider.Config{
    TagFilters: map[string]string{"cluster": "prod"},
})

// GCP — discovers instances in a zone:
import "github.com/tripleclabs/westcoast/pkg/gcpprovider"
provider := gcpprovider.New("my-project", "us-central1-a")

// Azure — discovers VMs in a resource group:
import "github.com/tripleclabs/westcoast/pkg/azureprovider"
provider := azureprovider.New("subscription-id", "my-resource-group")
```

### ClusterAuth

```go
type ClusterAuth interface {
    Credentials() ([]byte, error)
    Verify(peerCredentials []byte) error
}
```

Built-in implementations:
- `NoopAuth` — accepts all connections
- `SharedSecretAuth` — pre-shared secret with constant-time comparison
- `CertAuth` — x509 certificate verification against a cluster CA. Peer sends its DER-encoded certificate as credentials; verifier checks the certificate chain against the CA. Use with TLS-enabled transport for mTLS. `PeerNodeID(creds)` extracts the authenticated node ID from the certificate CN.

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

// Built-in adapters:
DirectPubSubAdapter     // fan-out to all nodes (simple, small clusters)
GossipPubSubAdapter     // batched gossip (larger clusters)
CRDTPubSubAdapter       // CRDT routing table — targeted sends only to interested nodes

// CRDT adapter setup:
adapter := cluster.NewCRDTPubSubAdapter(c, codec, crdtTransport, topology)
adapter.RegisterHandler(dispatcher)

rt := actor.NewRuntime(
    actor.WithPubSubBroadcast(adapter.Broadcast),
    actor.WithPubSubOnSubscribe(adapter.NotifySubscribe),
    actor.WithPubSubOnUnsubscribe(adapter.NotifyUnsubscribe),
)
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

### ClusterRouter

```go
type ClusterRouter struct { ... }

func NewClusterRouter(runtime *actor.Runtime, registry *DistributedRegistry, cluster ...*Cluster) *ClusterRouter
func (cr *ClusterRouter) Configure(serviceName string, strategy actor.RouterStrategy, opts ...RouterOption)
func (cr *ClusterRouter) Join(serviceName string, pid actor.PID) error
func (cr *ClusterRouter) Leave(serviceName string, pid actor.PID)
func (cr *ClusterRouter) Members(serviceName string) []actor.PID
func (cr *ClusterRouter) Send(ctx context.Context, serviceName string, payload any) actor.PIDSendAck
func (cr *ClusterRouter) SendWith(ctx context.Context, serviceName string, payload any, opts ...RoutePreference) actor.PIDSendAck
func (cr *ClusterRouter) Ask(ctx context.Context, serviceName string, payload any, timeout time.Duration) (actor.AskResult, error)
func (cr *ClusterRouter) AskWith(ctx context.Context, serviceName string, payload any, timeout time.Duration, opts ...RoutePreference) (actor.AskResult, error)
func (cr *ClusterRouter) Broadcast(ctx context.Context, serviceName string, payload any) []actor.PIDSendAck

// Configure-time options:
func WithWorkerFilter(m NodeMatcher) RouterOption
func WithWorkerPreference(r NodeRanker) RouterOption

// Call-time preferences:
func Nearest(tags map[string]string) RoutePreference      // locality scoring
func PreferTag(key string, direction RankDirection) RoutePreference  // numeric ranking
```

### SingletonSpec

```go
type SingletonSpec struct {
    Name         string              // actor ID and registered name
    InitialState any
    Handler      actor.Handler
    Options      []actor.ActorOption
    Placement    NodeMatcher         // nil = any node eligible

    // Handoff protocol (active when cluster/codec/dispatcher are provided):
    HandoffTimeout time.Duration                                    // default 10s
    CaptureState   func(ctx context.Context, actorID string) (any, error) // extract state on old leader
    OnHandoff      func(handoffState any) any                       // transform state on new leader
}

func NewSingletonManager(
    rt *actor.Runtime, election *RingElection, registry *DistributedRegistry,
    cluster *Cluster, codec Codec, dispatcher *InboundDispatcher,
) *SingletonManager
```

### DaemonSpec

```go
type DaemonSpec struct {
    Name         string           // actor ID — same on every node
    InitialState any
    Handler      actor.Handler
    Options      []actor.ActorOption
}

type DaemonSetManager struct { ... }

func NewDaemonSetManager(runtime *actor.Runtime, cluster *Cluster, codec Codec) *DaemonSetManager
func (dm *DaemonSetManager) Register(spec DaemonSpec)
func (dm *DaemonSetManager) Start(ctx context.Context)
func (dm *DaemonSetManager) Stop()
func (dm *DaemonSetManager) Running() []string
func (dm *DaemonSetManager) SendTo(ctx context.Context, name string, nodeID NodeID, payload any) actor.PIDSendAck
func (dm *DaemonSetManager) AskTo(ctx context.Context, name string, nodeID NodeID, payload any, timeout time.Duration) (actor.AskResult, error)
func (dm *DaemonSetManager) Broadcast(ctx context.Context, name string, payload any) []actor.PIDSendAck

// Replicated daemons — shared CRDT state across all instances:
type ReplicatedHandler[V any] func(ctx context.Context, state *crdt.ORMap[V], msg actor.Message) error
func RegisterReplicated[V any](dm *DaemonSetManager, name string, handler ReplicatedHandler[V], opts ...RegisterReplicatedOption)
```

### DrainConfig

```go
type DrainConfig struct {
    Timeout time.Duration // max wait for in-flight work; defaults to 30s
}

// Options:
func WithSingletonManager(sm *SingletonManager) DrainOption
func WithDaemonSetManager(dm *DaemonSetManager) DrainOption
func WithRegistry(r *DistributedRegistry) DrainOption
func WithPubSubAdapter(a *CRDTPubSubAdapter) DrainOption
func WithHandoffGrace(d time.Duration) DrainOption  // default 2s; time for singleton handoff requests
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
