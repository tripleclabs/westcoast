# CRDT-Actor Integration Ideas

Future features enabled by the `github.com/tripleclabs/crdt-go` library. These build on the registry migration (which replaces the in-tree `src/crdt/` with the full library).

## DurableActor

Actors whose state is a CRDT type. The runtime automatically replicates state mutations via deltas — no explicit messaging for state sync.

```go
rt.CreateDurableActor("user-cache", crdt.NewLWWMapReplica[User](nodeID, UserCodec{}),
    func(ctx context.Context, state *crdt.LWWMap[User], msg actor.Message) error {
        state.Put(msg.Key, msg.Payload.(User), replica.NextDot())
        return nil
    },
)
```

- State mutations produce deltas, which flow through the cluster transport to peer replicas
- Reads are always local (no round-trips)
- On actor restart, state recovers from the CRDT backend (memory or disk-backed)
- Clock strategy chosen per-actor (LWW, add-wins, etc.)
- Could support `DurableReplica` with write concerns for actors that need quorum confirmation

## ReplicatedDaemonSet

Extends `DaemonSetManager` so each daemon instance on every node shares a CRDT-backed state that converges automatically.

```go
dm.RegisterReplicated("session-store",
    crdt.NewAWLWWMapReplica[Session](nodeID, SessionCodec{}),
    handler,
)
```

- Each node runs a daemon instance with its own CRDT replica
- Mutations on any node produce deltas gossiped to all other instances
- Add-wins semantics: concurrent session creation on different nodes both survive
- Natural fit for: session stores, feature flags, routing tables, circuit breaker state

## Distributed Counters / Gauges

First-class `GCounter` and `PNCounter` primitives exposed through the actor system.

Use cases:
- **Rate limiting**: PNCounter tracks "tokens remaining" — decremented locally, converges cluster-wide
- **Distributed metrics**: GCounter for request counts, error counts across nodes — read locally, aggregate eventually
- **Capacity tracking**: PNCounter for "available slots" across a worker pool spanning nodes
- **Quota enforcement**: decrement locally, periodic anti-entropy ensures global convergence

```go
counter := cluster.NewDistributedCounter("api-requests", nodeID)
counter.Increment(1)          // local, no round-trip
total := counter.Value()      // eventually consistent cluster-wide total
```

## Replicated PubSub Metadata

Replace the current gossip-based subscription tracking with an `LWWMap` replica.

- Subscribers register with their node — the map entry is `topic+nodeID → subscribed/unsubscribed`
- The LWWMap converges across nodes via delta gossip
- Publish routing uses local reads (no cross-node lookup for "who is subscribed?")
- Tombstones handle unsubscribe correctly (LWW semantics)
- Anti-entropy via `DeltasSince` handles partition recovery

## Cluster-Wide Configuration Store

An `AWLWWMap` replica as a distributed config map accessible from any actor.

```go
cfg := cluster.NewConfigStore(nodeID)
cfg.Put("feature.new-ui", "true")        // write from any node
val, _ := cfg.Get("feature.new-ui")      // local read, eventually consistent
```

- Add-wins semantics: concurrent puts both survive, concurrent put+delete → put wins
- Operators write from any node; all actors read locally
- Watch API: actors subscribe to key changes via PubSub

## MVRegister for Conflict-Aware Actors

Actors that need to detect and resolve concurrent writes rather than silently LWW-ing.

```go
reg := crdt.NewMVRegisterReplica[Order](nodeID, OrderCodec{})
// After concurrent writes from two nodes:
values, _ := reg.Data.Values()  // returns both concurrent versions
// Actor decides how to merge (e.g. combine line items, pick higher total)
```

Use cases:
- Shopping cart merging (concurrent adds on different nodes)
- Document editing with conflict detection
- Inventory systems where concurrent reservations need explicit resolution

## MerkleMap for Efficient Anti-Entropy

Use `MerkleMap` for cheap divergence detection before running full anti-entropy.

- Each node maintains a MerkleMap over its CRDT state
- Periodic comparison with peers: if roots match, skip sync entirely
- On divergence: `DivergentKeys()` identifies exactly which keys differ
- Only sync divergent keys via `DeltasSince` — minimal bandwidth

This would replace the current digest-based comparison in `registry_gossip.go` with something that scales to much larger state.

## Write-Concern Actors

Actors backed by `DurableReplica` that block until a quorum of peers has the write.

```go
rt.CreateDurableActor("lock-manager", replica,
    handler,
    actor.WithWriteConcern(crdt.WMajority),
)
```

- Still a CRDT — always available, always convergent
- Write concern adds propagation guarantee (N peers confirmed)
- On timeout: write is locally applied, anti-entropy syncs later
- Use cases: distributed lock coordinators, leader state, critical config updates
