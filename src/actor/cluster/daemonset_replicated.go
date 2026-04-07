package cluster

import (
	"bytes"
	"context"
	"encoding/gob"

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
func (c *Cluster) RegisterReplicated(name string, handler ReplicatedHandler[any], opts ...RegisterReplicatedOption) {
	registerReplicated[any](c.daemonMgr, name, handler, opts...)
}

// registerReplicated is the internal generic implementation. The public
// Cluster method uses [any]; the package-level function preserves type
// safety for callers who want it.
func registerReplicated[V any](dm *DaemonSetManager, name string, handler ReplicatedHandler[V], opts ...RegisterReplicatedOption) {
	var ro replicatedOpts
	for _, o := range opts {
		o(&ro)
	}

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
				m := crdt.NewORMap[V](replicaID, gobCodec[V]{}, crdtOpts...)
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

// gobCodec is a crdt.Codec[V] that uses gob encoding, matching the
// serialization used throughout westcoast.
type gobCodec[V any] struct{}

func (gobCodec[V]) Encode(v V) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (gobCodec[V]) Decode(data []byte) (V, error) {
	var v V
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&v); err != nil {
		var zero V
		return zero, err
	}
	return v, nil
}
