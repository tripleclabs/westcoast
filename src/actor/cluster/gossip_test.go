package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestGossipProtocol_ProducesAndSends(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()

	transport1 := newTestTransport("node-1")
	transport2 := newTestTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	received := make(chan []byte, 64)
	c1, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := testTransportAddr(transport1)
	addr2 := testTransportAddr(transport2)
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	cfg := GossipConfig{
		Interval: 50 * time.Millisecond,
		Fanout:   3,
		Protocol: "test",
		OnProduce: func() []byte {
			return []byte("hello")
		},
	}

	g1 := NewGossipProtocol(c1, codec, cfg)

	g2Cfg := GossipConfig{
		Protocol: "test",
		OnReceive: func(from NodeID, data []byte) {
			received <- data
		},
	}
	g2 := NewGossipProtocol(c2, codec, g2Cfg)

	d1 := NewInboundDispatcher(nil, codec)
	d2 := NewInboundDispatcher(nil, codec)
	RegisterGossipHandler(d1, codec, g1)
	RegisterGossipHandler(d2, codec, g2)

	c1.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d1.Dispatch(ctx, from, env)
	}
	c2.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d2.Dispatch(ctx, from, env)
	}

	g1.Start(ctx)
	defer g1.Stop()
	g2.Start(ctx)
	defer g2.Stop()

	select {
	case data := <-received:
		if string(data) != "hello" {
			t.Errorf("got %q, want hello", data)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for gossip")
	}
}

func TestGossipProtocol_SelectsRandomPeers(t *testing.T) {
	members := []NodeMeta{
		{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"},
	}
	peers := selectRandomPeers(members, 2)
	if len(peers) != 2 {
		t.Errorf("expected 2, got %d", len(peers))
	}
}

func TestDistributedRegistry_ReplicatesViaCRDTTransport(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register(actor.PID{})

	transport1 := newTestTransport("node-1")
	transport2 := newTestTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self:     NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	// Create CRDT transports.
	ct1 := NewClusterCRDTTransport(c1)
	ct2 := NewClusterCRDTTransport(c2)

	topo1 := NewClusterTopology(c1)
	topo2 := NewClusterTopology(c2)

	// Create registries backed by CRDT replication.
	reg1 := NewDistributedRegistryWithTransport("node-1", ct1, topo1)
	reg2 := NewDistributedRegistryWithTransport("node-2", ct2, topo2)
	defer reg1.Close()
	defer reg2.Close()

	// Wire inbound dispatch.
	d1 := NewInboundDispatcher(nil, codec)
	d2 := NewInboundDispatcher(nil, codec)
	ct1.RegisterHandler(d1)
	ct2.RegisterHandler(d2)

	c1.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d1.Dispatch(ctx, from, env)
	}
	c2.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d2.Dispatch(ctx, from, env)
	}

	// Start clusters and connect.
	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := testTransportAddr(transport1)
	addr2 := testTransportAddr(transport2)
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Register on node-1 — should replicate to node-2 automatically.
	reg1.Register("svc-a", actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1})

	converged := false
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, ok := reg2.Lookup("svc-a"); ok {
			converged = true
			break
		}
	}
	if !converged {
		t.Fatal("registry did not replicate from node-1 to node-2")
	}
}
