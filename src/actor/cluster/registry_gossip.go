package cluster

import (
	"bytes"
	"context"
	"encoding/gob"

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
// A gossip round works as follows:
//  1. Initiator sends a message with IsDigest=true, containing their Digest.
//  2. Responder compares, computes the delta, and calls MergeDelta on itself
//     with the initiator's entries (which it receives in a subsequent round).
//
// For simplicity we use a push model: each round, the node sends its full
// state delta to peers based on an empty digest (i.e. "assume the peer has
// nothing new"). The peer merges and the digest comparison on the NEXT round
// ensures convergence. This is simpler than the request-response pattern
// and works well with the periodic gossip model.
type registryGossipMsg struct {
	Delta crdt.StateDelta
}

const registryGossipProtocol = "crdt_registry"

// RegistryGossip connects a CRDTRegistry to the GossipProtocol for
// automatic state synchronization across the cluster.
type RegistryGossip struct {
	registry *CRDTRegistry
	gossip   *GossipProtocol
}

// NewRegistryGossip creates and starts a gossip-backed registry sync.
// The gossip protocol will periodically push state deltas to peers and
// merge incoming deltas.
func NewRegistryGossip(registry *CRDTRegistry, cluster *Cluster, codec Codec, cfg GossipConfig) *RegistryGossip {
	rg := &RegistryGossip{registry: registry}

	cfg.Protocol = registryGossipProtocol
	cfg.OnProduce = rg.produce
	cfg.OnReceive = rg.receive

	rg.gossip = NewGossipProtocol(cluster, codec, cfg)
	return rg
}

// Start begins periodic gossip.
func (rg *RegistryGossip) Start(ctx context.Context) {
	rg.gossip.Start(ctx)
}

// Stop terminates gossip.
func (rg *RegistryGossip) Stop() {
	rg.gossip.Stop()
}

// GossipProtocol returns the underlying gossip protocol for inbound routing.
func (rg *RegistryGossip) GossipProtocol() *GossipProtocol {
	return rg.gossip
}

// produce generates the gossip payload: a delta computed against an empty
// digest (i.e. our full state). Peers merge what they're missing.
func (rg *RegistryGossip) produce() []byte {
	// Compute delta against empty digest — this gives us all our entries.
	// Peers will ignore entries they already have (same tag).
	delta := rg.registry.DeltaFor(crdt.Digest{})
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
