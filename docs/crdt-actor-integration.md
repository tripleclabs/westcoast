# CRDT-Actor Integration Ideas

Future features enabled by the `github.com/tripleclabs/crdt-go` library.

## Implemented

### Distributed Registry

`DistributedRegistry` backed by `crdt.ORMap[actor.PID]`. Replication and anti-entropy handled entirely by the CRDT library — we just implement `crdt.Transport` to bridge to `SendRemote`.

### Replicated DaemonSet State

`RegisterReplicated[V]` on `DaemonSetManager`. Each daemon instance gets a `crdt.ORMap[V]` that converges automatically. Topology is placement-aware — only nodes running the daemon participate in replication.

```go
cluster.RegisterReplicated[Session](dm, "session-store",
    func(ctx context.Context, sessions *crdt.ORMap[Session], msg actor.Message) error {
        sessions.Put(ctx, msg.Key, msg.Payload.(Session))
        return nil
    },
)
```

## Future Ideas

### Distributed Counters / Gauges

First-class `GCounter` and `PNCounter` exposed through the actor system.

```go
counter := cluster.NewDistributedCounter("api-requests",
    crdt.WithTransport(transport),
    crdt.WithTopology(topology),
)
counter.Increment(ctx, 1)    // local, no round-trip
total := counter.Int64()     // eventually consistent cluster-wide total
```

Use cases: rate limiting, distributed metrics, capacity tracking, quota enforcement.

### Replicated PubSub Metadata

Replace gossip-based subscription tracking with a `crdt.LWWMap`. Subscribers register locally, the map converges across nodes, publish routing uses local reads.

### Cluster-Wide Configuration Store

`crdt.ORMap` as a distributed config map. Operators write from any node, actors read locally. Could add a watch API via PubSub for key-change notifications.

```go
cfg := cluster.NewConfigStore(transport, topology)
cfg.Put(ctx, "feature.new-ui", "true")
val, _ := cfg.Get("feature.new-ui")
```

### MVRegister for Conflict-Aware Actors

Actors that need to detect concurrent writes rather than silently resolving them. `crdt.MVRegister` preserves all concurrent values — the actor decides how to merge.

Use cases: shopping cart merging, document editing, inventory reservation conflicts.

### Write-Concern Daemons

`RegisterReplicated` with `crdt.WithWriteConcern(crdt.WMajority)` — the `Put` call blocks until a quorum of peer instances have the write. Still a CRDT, still available, but with a propagation guarantee.

```go
cluster.RegisterReplicated[LockState](dm, "lock-manager", handler,
    crdt.WithWriteConcern(crdt.WMajority),
)
```
