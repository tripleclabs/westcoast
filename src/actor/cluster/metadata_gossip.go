package cluster

import (
	"bytes"
	"context"
	"encoding/gob"
)

func init() {
	gob.Register(metadataGossipMsg{})
}

const metadataGossipProtocol = "node_metadata"

// metadataGossipMsg carries a node's current metadata for gossip.
type metadataGossipMsg struct {
	Meta NodeMeta
}

// MetadataGossip periodically gossips this node's metadata (including
// dynamic tags) to peers. When a peer's metadata changes, the cluster's
// peer map is updated and a MemberUpdated event is emitted.
type MetadataGossip struct {
	cluster *Cluster
	gossip  *GossipProtocol
}

// NewMetadataGossip creates and configures a metadata gossip protocol.
func NewMetadataGossip(cluster *Cluster, codec Codec, cfg GossipConfig) *MetadataGossip {
	mg := &MetadataGossip{cluster: cluster}

	cfg.Protocol = metadataGossipProtocol
	cfg.OnProduce = mg.produce
	cfg.OnReceive = mg.receive

	mg.gossip = NewGossipProtocol(cluster, codec, cfg)
	return mg
}

// Start begins periodic metadata gossip.
func (mg *MetadataGossip) Start(ctx context.Context) {
	mg.gossip.Start(ctx)
}

// Stop terminates metadata gossip.
func (mg *MetadataGossip) Stop() {
	mg.gossip.Stop()
}

// GossipProtocol returns the underlying gossip for inbound routing.
func (mg *MetadataGossip) GossipProtocol() *GossipProtocol {
	return mg.gossip
}

func (mg *MetadataGossip) produce() []byte {
	meta := mg.cluster.Self()
	msg := metadataGossipMsg{Meta: meta}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(msg); err != nil {
		return nil
	}
	return buf.Bytes()
}

func (mg *MetadataGossip) receive(from NodeID, data []byte) {
	var msg metadataGossipMsg
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&msg); err != nil {
		return
	}
	mg.cluster.UpdatePeerMeta(msg.Meta)
}

// RegisterMetadataGossip adds the metadata gossip protocol to a GossipRouter.
func RegisterMetadataGossip(router *GossipRouter, mg *MetadataGossip) {
	router.Add(mg.GossipProtocol())
}
