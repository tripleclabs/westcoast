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

// RegisterReplicated registers a daemon that runs on every matching node with
// shared CRDT-replicated state. Each instance gets an ORMap[V] that
// converges automatically across the cluster — no explicit sync needed.
//
// The type parameter V is the map's value type. Values are gob-encoded
// internally. The handler receives the live ORMap for reads and writes.
//
// Example:
//
//	dm.RegisterReplicated[Session]("session-store",
//	    func(ctx context.Context, sessions *crdt.ORMap[Session], msg actor.Message) error {
//	        switch m := msg.Payload.(type) {
//	        case CreateSession:
//	            sessions.Put(ctx, m.ID, m.Session)
//	        case GetSession:
//	            s, ok := sessions.Get(m.ID)
//	            // ...
//	        }
//	        return nil
//	    },
//	)
func RegisterReplicated[V any](dm *DaemonSetManager, name string, handler ReplicatedHandler[V], placement ...NodeMatcher) {
	var p NodeMatcher
	if len(placement) > 0 {
		p = placement[0]
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.daemons[name] = &daemonState{
		spec: DaemonSpec{
			Name:      name,
			Placement: p,
		},
		replicated: &replicatedConfig{
			create: func(replicaID crdt.ReplicaID, transport crdt.Transport, topology crdt.TopologyProvider) (state any, actorHandler actor.Handler, closer func()) {
				var opts []crdt.Option
				if transport != nil {
					opts = append(opts, crdt.WithTransport(transport))
				}
				if topology != nil {
					opts = append(opts, crdt.WithTopology(topology))
				}
				m := crdt.NewORMap[V](replicaID, gobCodec[V]{}, opts...)
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
