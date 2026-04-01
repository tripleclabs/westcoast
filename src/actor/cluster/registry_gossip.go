package cluster

import (
	"bytes"
	"context"
	"encoding/gob"
	"sync"

	"github.com/tripleclabs/westcoast/src/crdt"
)

func init() {
	gob.Register(registryGossipMsg{})
	gob.Register(crdt.Digest{})
	gob.Register(crdt.StateDelta{})
	gob.Register(crdt.Entry{})
	gob.Register(crdt.Tag(""))
	gob.Register(crdt.VectorTimestamp{})
}

// registryGossipMsg is the wire format for registry gossip.
// Contains only the entries/tombstones that changed since the last round.
type registryGossipMsg struct {
	Delta crdt.StateDelta
}

const registryGossipProtocol = "crdt_registry"

// RegistryGossip connects a CRDTRegistry to the GossipProtocol for
// automatic state synchronization across the cluster.
//
// Each gossip round, it computes the diff between the current registry
// state and the last state it sent, and only transmits the changes.
// Peers merge the incoming delta. Convergence happens over multiple
// rounds as each side pushes its diffs.
type RegistryGossip struct {
	registry *CRDTRegistry
	gossip   *GossipProtocol

	mu         sync.Mutex
	lastDigest crdt.Digest // snapshot of what we sent last round
	roundCount uint64      // counts gossip rounds
}

// fullSyncEvery controls how often a full state push is done to catch
// peers that missed a diff round. Every N rounds, send full state.
const fullSyncEvery = 10

// NewRegistryGossip creates a RegistryGossip that synchronizes the given registry over the cluster's gossip protocol.
func NewRegistryGossip(registry *CRDTRegistry, cluster *Cluster, codec Codec, cfg GossipConfig) *RegistryGossip {
	rg := &RegistryGossip{
		registry:   registry,
		lastDigest: crdt.Digest{},
	}

	cfg.Protocol = registryGossipProtocol
	cfg.OnProduce = rg.produce
	cfg.OnReceive = rg.receive

	rg.gossip = NewGossipProtocol(cluster, codec, cfg)
	return rg
}

// Start begins the gossip protocol for registry synchronization.
func (rg *RegistryGossip) Start(ctx context.Context) {
	rg.gossip.Start(ctx)
}

// Stop shuts down the gossip protocol for registry synchronization.
func (rg *RegistryGossip) Stop() {
	rg.gossip.Stop()
}

// GossipProtocol returns the underlying gossip protocol instance.
func (rg *RegistryGossip) GossipProtocol() *GossipProtocol {
	return rg.gossip
}

// produce generates the gossip payload: only entries that changed since
// the last round. This is O(delta) per round instead of O(N).
func (rg *RegistryGossip) produce() []byte {
	rg.mu.Lock()
	defer rg.mu.Unlock()

	rg.roundCount++
	currentDigest := rg.registry.Digest()

	var delta crdt.StateDelta
	if rg.roundCount%fullSyncEvery == 0 {
		// Periodic full sync to catch peers that missed a diff round.
		delta = rg.registry.DeltaFor(crdt.Digest{})
	} else {
		// Normal: only send what changed since last round.
		delta = rg.registry.DeltaFor(rg.lastDigest)
	}

	rg.lastDigest = currentDigest

	if len(delta.Entries) == 0 && len(delta.Tombstones) == 0 {
		return nil
	}

	msg := registryGossipMsg{Delta: delta}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return nil
	}
	return buf.Bytes()
}

// receive processes an incoming gossip payload from a peer.
func (rg *RegistryGossip) receive(from NodeID, data []byte) {
	var msg registryGossipMsg
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&msg); err != nil {
		return
	}
	rg.registry.MergeDelta(msg.Delta)
}
