package cluster

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"hash/fnv"

	crdt "github.com/tripleclabs/crdt-go"
	"github.com/tripleclabs/westcoast/src/actor"
)

// pidCodec encodes actor.PID values for the CRDT ORMap.
type pidCodec struct{}

func (pidCodec) Encode(pid actor.PID) ([]byte, error) {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(pid); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (pidCodec) Decode(data []byte) (actor.PID, error) {
	var pid actor.PID
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&pid); err != nil {
		return actor.PID{}, err
	}
	return pid, nil
}

// nodeIDToReplicaID produces a deterministic uint64 replica ID from a string
// NodeID using FNV-1a hashing.
func nodeIDToReplicaID(id NodeID) crdt.ReplicaID {
	h := fnv.New64a()
	h.Write([]byte(id))
	return h.Sum64()
}

// DistributedRegistry is a distributed name registry backed by a crdt.ORMap
// with add-wins semantics. Replication and anti-entropy are handled entirely
// by the CRDT library — just wire up a Transport and TopologyProvider.
type DistributedRegistry struct {
	data *crdt.ORMap[actor.PID]
}

// NewDistributedRegistry creates a distributed registry for single-node use
// or testing. No replication.
func NewDistributedRegistry(nodeID NodeID) *DistributedRegistry {
	replicaID := nodeIDToReplicaID(nodeID)
	return &DistributedRegistry{
		data: crdt.NewORMap[actor.PID](replicaID, pidCodec{}),
	}
}

// NewDistributedRegistryWithTransport creates a distributed registry with
// automatic CRDT replication and anti-entropy.
func NewDistributedRegistryWithTransport(nodeID NodeID, transport crdt.Transport, topology crdt.TopologyProvider, opts ...crdt.Option) *DistributedRegistry {
	replicaID := nodeIDToReplicaID(nodeID)
	allOpts := append([]crdt.Option{
		crdt.WithTransport(transport),
		crdt.WithTopology(topology),
	}, opts...)
	return &DistributedRegistry{
		data: crdt.NewORMap[actor.PID](replicaID, pidCodec{}, allOpts...),
	}
}

func (r *DistributedRegistry) Register(name string, pid actor.PID) error {
	if existing, ok := r.data.Get(name); ok {
		if existing.Key() == pid.Key() {
			return nil // idempotent
		}
		return errors.New("name already registered")
	}
	_, err := r.data.Put(context.Background(), name, pid)
	return err
}

func (r *DistributedRegistry) Lookup(name string) (actor.PID, bool) {
	return r.data.Get(name)
}

func (r *DistributedRegistry) Unregister(name string) (actor.PID, bool) {
	pid, ok := r.data.Get(name)
	if !ok {
		return actor.PID{}, false
	}
	r.data.Remove(context.Background(), name)
	return pid, true
}

// UnregisterByNode removes all entries where the PID lives on the given node.
func (r *DistributedRegistry) UnregisterByNode(node NodeID) []string {
	var toRemove []string
	r.data.Range(func(key string, pid actor.PID) bool {
		if pid.Namespace == string(node) {
			toRemove = append(toRemove, key)
		}
		return true
	})
	for _, name := range toRemove {
		r.data.Remove(context.Background(), name)
	}
	return toRemove
}

func (r *DistributedRegistry) OnMembershipChange(event MemberEvent) {
	if event.Type == MemberFailed || event.Type == MemberLeave {
		r.UnregisterByNode(event.Member.ID)
	}
}

// Range calls fn for each registered name and PID.
func (r *DistributedRegistry) Range(fn func(name string, pid actor.PID) bool) {
	r.data.Range(fn)
}

// NamesByNode returns all names where the PID lives on the given node.
func (r *DistributedRegistry) NamesByNode(node NodeID) []string {
	var names []string
	r.data.Range(func(name string, pid actor.PID) bool {
		if pid.Namespace == string(node) {
			names = append(names, name)
		}
		return true
	})
	return names
}

// AllEntries returns all registered name→PID bindings.
func (r *DistributedRegistry) AllEntries() map[string]actor.PID {
	out := make(map[string]actor.PID)
	r.data.Range(func(name string, pid actor.PID) bool {
		out[name] = pid
		return true
	})
	return out
}

// Close shuts down replication and anti-entropy.
func (r *DistributedRegistry) Close() {
	r.data.Close()
}

