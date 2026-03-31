package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestGossipProtocol_ProducesAndSends(t *testing.T) {
	ctx := context.Background()

	// Set up a 2-node cluster.
	codec := NewGobCodec()
	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	var mu sync.Mutex
	var received []Envelope

	c1, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
		OnEnvelope: func(from NodeID, env Envelope) {
			mu.Lock()
			received = append(received, env)
			mu.Unlock()
		},
	})

	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr2 := transport2.listener.Addr().String()
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Wait for connection.
	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(20 * time.Millisecond):
		}
	}

	// Create a gossip protocol that produces a static payload.
	produced := false
	g := NewGossipProtocol(c1, codec, GossipConfig{
		Protocol: "test",
		Interval: 50 * time.Millisecond,
		Fanout:   3,
		OnProduce: func() []byte {
			if produced {
				return nil // only produce once
			}
			produced = true
			return []byte("hello-gossip")
		},
		OnReceive: func(from NodeID, data []byte) {},
	})

	g.Start(ctx)
	defer g.Stop()

	// Wait for at least one gossip round.
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	count := len(received)
	mu.Unlock()

	if count == 0 {
		t.Error("node-2 should have received at least one gossip envelope")
	}
	if !produced {
		t.Error("OnProduce should have been called")
	}
}

func TestGossipProtocol_HandleInbound(t *testing.T) {
	var receivedData []byte
	var receivedFrom NodeID

	g := &GossipProtocol{
		protocol:  "test-proto",
		onReceive: func(from NodeID, data []byte) {
			receivedFrom = from
			receivedData = data
		},
	}

	g.HandleInbound("node-2", GossipEnvelope{
		Protocol: "test-proto",
		Payload:  []byte("payload"),
	})

	if string(receivedData) != "payload" {
		t.Errorf("expected payload, got %s", receivedData)
	}
	if receivedFrom != "node-2" {
		t.Errorf("expected node-2, got %s", receivedFrom)
	}

	// Wrong protocol should be ignored.
	receivedData = nil
	g.HandleInbound("node-3", GossipEnvelope{
		Protocol: "wrong-proto",
		Payload:  []byte("ignored"),
	})
	if receivedData != nil {
		t.Error("should ignore wrong protocol")
	}
}

func TestGossipProtocol_SelectRandomPeers(t *testing.T) {
	members := makeMembers("a", "b", "c", "d", "e")

	// Selecting more than available should return all.
	peers := selectRandomPeers(members, 10)
	if len(peers) != 5 {
		t.Errorf("expected 5, got %d", len(peers))
	}

	// Selecting fewer should return exactly N.
	peers = selectRandomPeers(members, 2)
	if len(peers) != 2 {
		t.Errorf("expected 2, got %d", len(peers))
	}
}

func TestRegistryGossip_ConvergesViaGossip(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()
	codec.Register(actor.PID{})

	// Set up 2-node cluster.
	transport1 := NewGRPCTransport("node-1")
	transport2 := NewGRPCTransport("node-2")
	provider1 := NewFixedProvider(FixedProviderConfig{})
	provider2 := NewFixedProvider(FixedProviderConfig{})

	c1, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider: provider1, Transport: transport1,
		Auth: NoopAuth{}, Codec: codec,
	})
	c2, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider: provider2, Transport: transport2,
		Auth: NoopAuth{}, Codec: codec,
	})

	// Create registries.
	reg1 := NewCRDTRegistry("node-1")
	reg2 := NewCRDTRegistry("node-2")

	// Create gossip-backed sync.
	gossipCfg := GossipConfig{Interval: 50 * time.Millisecond, Fanout: 3}
	rg1 := NewRegistryGossip(reg1, c1, codec, gossipCfg)
	rg2 := NewRegistryGossip(reg2, c2, codec, gossipCfg)

	// Wire inbound routing via InboundDispatcher with registered gossip handler.
	d1 := NewInboundDispatcher(nil, codec)
	d2 := NewInboundDispatcher(nil, codec)

	RegisterGossipHandler(d1, codec, rg1.GossipProtocol())
	RegisterGossipHandler(d2, codec, rg2.GossipProtocol())

	c1.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d1.Dispatch(ctx, from, env)
	}
	c2.cfg.OnEnvelope = func(from NodeID, env Envelope) {
		d2.Dispatch(ctx, from, env)
	}

	// Start everything.
	c1.Start(ctx)
	c2.Start(ctx)
	defer c1.Stop()
	defer c2.Stop()

	addr1 := transport1.listener.Addr().String()
	addr2 := transport2.listener.Addr().String()
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

	// Register entries on each node.
	reg1.Register("svc-a", actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1})
	reg2.Register("svc-b", actor.PID{Namespace: "node-2", ActorID: "b", Generation: 1})

	// Start gossip.
	rg1.Start(ctx)
	rg2.Start(ctx)
	defer rg1.Stop()
	defer rg2.Stop()

	// Wait for convergence (a few gossip rounds).
	converged := false
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		_, hasA := reg2.Lookup("svc-a")
		_, hasB := reg1.Lookup("svc-b")
		if hasA && hasB {
			converged = true
			break
		}
	}

	if !converged {
		all1 := reg1.AllEntries()
		all2 := reg2.AllEntries()
		t.Fatalf("registries did not converge via gossip.\nreg1: %v\nreg2: %v", all1, all2)
	}
}
