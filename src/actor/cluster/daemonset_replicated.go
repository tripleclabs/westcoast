package cluster

import (
	"context"

	crdt "github.com/tripleclabs/crdt-go"
	"github.com/tripleclabs/westcoast/src/actor"
)

// ReplicatedHandler is the handler signature for replicated daemon actors.
// The state is a *crdt.ORMap[V] with add-wins semantics — reads are local,
// writes replicate automatically to all peer instances.
type ReplicatedHandler[V any] func(ctx context.Context, state *crdt.ORMap[V], msg actor.Message) error

// RegisterReplicatedOption configures a replicated daemon registration.
type RegisterReplicatedOption func(*replicatedOpts)

type replicatedOpts struct {
	placement   NodeMatcher
	crdtOptions []crdt.Option
}

// WithReplicatedPlacement restricts which nodes run this replicated daemon.
func WithReplicatedPlacement(m NodeMatcher) RegisterReplicatedOption {
	return func(o *replicatedOpts) { o.placement = m }
}

// WithCRDTOptions passes extra crdt.Option values to the ORMap constructor.
// Use this to inject a persistent backend (e.g. crdtbolt), write concern, or
// anti-entropy interval.
func WithCRDTOptions(opts ...crdt.Option) RegisterReplicatedOption {
	return func(o *replicatedOpts) { o.crdtOptions = append(o.crdtOptions, opts...) }
}

// RegisterReplicated registers a daemon that runs on every matching node with
// shared CRDT-replicated state. Each instance gets an ORMap that converges
// automatically across the cluster — no explicit sync needed.
//
// Values are gob-encoded for replication. The handler receives the live
// ORMap for reads and writes.
//
// Example:
//
//	c.RegisterReplicated("session-store",
//	    func(ctx context.Context, sessions *crdt.ORMap[any], msg actor.Message) error {
//	        switch m := msg.Payload.(type) {
//	        case CreateSession:
//	            sessions.Put(ctx, m.ID, m.Session)
//	        case GetSession:
//	            s, _ := sessions.Get(m.ID)
//	            // ...
//	        }
//	        return nil
//	    },
//	)
// RegisterReplicated registers a daemon with CRDT-replicated typed state.
// The codec handles serialization for wire transport — provide one that
// matches your value type. The crdt package provides built-in codecs:
// [crdt.StringCodec], [crdt.Int64Codec], [crdt.BytesCodec].
//
// This is a package function (not a method) because Go does not allow
// type parameters on methods.
//
// Example:
//
//	cluster.RegisterReplicated(c, "session-store", sessionCodec{},
//	    func(ctx context.Context, sessions *crdt.ORMap[Session], msg actor.Message) error {
//	        switch m := msg.Payload.(type) {
//	        case CreateSession:
//	            sessions.Put(ctx, m.ID, m.Session)
//	        case GetSession:
//	            s, _ := sessions.Get(m.ID)
//	            // use s directly — fully typed
//	        }
//	        return nil
//	    },
//	)
func RegisterReplicated[V any](c *Cluster, name string, codec crdt.Codec[V], handler ReplicatedHandler[V], opts ...RegisterReplicatedOption) {
	var ro replicatedOpts
	for _, o := range opts {
		o(&ro)
	}

	dm := c.daemonMgr
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.daemons[name] = &daemonState{
		spec: DaemonSpec{
			Name:      name,
			Placement: ro.placement,
		},
		replicated: &replicatedConfig{
			create: func(replicaID crdt.ReplicaID, transport crdt.Transport, topology crdt.TopologyProvider) (state any, actorHandler actor.Handler, closer func()) {
				var crdtOpts []crdt.Option
				if transport != nil {
					crdtOpts = append(crdtOpts, crdt.WithTransport(transport))
				}
				if topology != nil {
					crdtOpts = append(crdtOpts, crdt.WithTopology(topology))
				}
				crdtOpts = append(crdtOpts, ro.crdtOptions...)
				m := crdt.NewORMap[V](replicaID, codec, crdtOpts...)
				actorHandler = func(ctx context.Context, state any, msg actor.Message) (any, error) {
					err := handler(ctx, state.(*crdt.ORMap[V]), msg)
					return state, err
				}
				return m, actorHandler, m.Close
			},
		},
	}

	if dm.started {
		dm.reconcileDaemon(name)
	}
}

// replicatedConfig holds the factory for creating CRDT-backed state.
// Stored in daemonState for replicated daemons.
type replicatedConfig struct {
	create func(replicaID crdt.ReplicaID, transport crdt.Transport, topology crdt.TopologyProvider) (state any, handler actor.Handler, closer func())
}

