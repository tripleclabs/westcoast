package cluster

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCluster_TwoNodeFormation(t *testing.T) {
	ctx := context.Background()
	received := make(chan Envelope, 64)

	// Node 1.
	provider1 := NewFixedProvider(FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	transport1 := NewGRPCTransport("node-1")
	cluster1, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}

	if err := cluster1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer cluster1.Stop()

	// Get actual listen address.
	addr1 := transport1.listener.Addr().String()

	// Node 2.
	provider2 := NewFixedProvider(FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	transport2 := NewGRPCTransport("node-2")
	cluster2, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
		OnEnvelope: func(from NodeID, env Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}

	if err := cluster2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer cluster2.Stop()

	addr2 := transport2.listener.Addr().String()

	// Manually tell each provider about the other node.
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	// Wait for connections to establish.
	deadline := time.After(3 * time.Second)
	for !cluster1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cluster1 to connect to node-2")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Send an envelope from node-1 to node-2.
	env := Envelope{
		SenderNode:    "node-1",
		SenderActorID: "actor-a",
		TargetNode:    "node-2",
		TargetActorID: "actor-b",
		MessageID:     42,
		Payload:       []byte("hello from node-1"),
	}
	if err := cluster1.SendRemote(ctx, "node-2", env); err != nil {
		t.Fatalf("send remote: %v", err)
	}

	// Node-2 should receive it via OnEnvelope callback.
	select {
	case got := <-received:
		if got.TargetActorID != "actor-b" {
			t.Errorf("target actor: got %s, want actor-b", got.TargetActorID)
		}
		if string(got.Payload) != "hello from node-1" {
			t.Errorf("payload: got %s", got.Payload)
		}
		if got.MessageID != 42 {
			t.Errorf("message ID: got %d, want 42", got.MessageID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for envelope delivery")
	}
}

func TestCluster_MembershipEvents(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")

	c, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster: %v", err)
	}
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	// Add a member.
	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:19999"})

	// Wait for the cluster to process the event.
	time.Sleep(100 * time.Millisecond)

	members := c.Members()
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if members[0].ID != "node-2" {
		t.Errorf("expected node-2, got %s", members[0].ID)
	}
}

func TestCluster_SendToUnknownNodeFails(t *testing.T) {
	ctx := context.Background()

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")

	c, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster: %v", err)
	}
	if err := c.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer c.Stop()

	err = c.SendRemote(ctx, "unknown-node", Envelope{Payload: []byte("test")})
	if err == nil {
		t.Fatal("expected error sending to unknown node")
	}
}

func TestCluster_LocalNodeID(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("my-node")

	c, err := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "my-node", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
	})
	if err != nil {
		t.Fatalf("new cluster: %v", err)
	}

	if c.LocalNodeID() != "my-node" {
		t.Errorf("got %s, want my-node", c.LocalNodeID())
	}
}

func TestCluster_MultipleEnvelopesDelivered(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var received []Envelope

	provider1 := NewFixedProvider(FixedProviderConfig{})
	transport1 := NewGRPCTransport("node-1")
	c1, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c1.Start(ctx)
	defer c1.Stop()
	addr1 := transport1.listener.Addr().String()
	_ = addr1

	provider2 := NewFixedProvider(FixedProviderConfig{})
	transport2 := NewGRPCTransport("node-2")
	c2, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
		OnEnvelope: func(from NodeID, env Envelope) {
			mu.Lock()
			received = append(received, env)
			mu.Unlock()
		},
	})
	c2.Start(ctx)
	defer c2.Stop()
	addr2 := transport2.listener.Addr().String()

	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Wait for connection.
	deadline := time.After(3 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Send 20 envelopes.
	const count = 20
	for i := 0; i < count; i++ {
		c1.SendRemote(ctx, "node-2", Envelope{
			MessageID: uint64(i),
			Payload:   []byte("msg"),
		})
	}

	// Wait for delivery.
	deadline = time.After(3 * time.Second)
	for {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n >= count {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("only received %d/%d envelopes", len(received), count)
			mu.Unlock()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func TestNewCluster_ValidationErrors(t *testing.T) {
	transport := NewGRPCTransport("n")
	provider := NewFixedProvider(FixedProviderConfig{})

	cases := []struct {
		name string
		cfg  ClusterConfig
	}{
		{"missing ID", ClusterConfig{Self: NodeMeta{Addr: ":0"}, Provider: provider, Transport: transport}},
		{"missing Addr", ClusterConfig{Self: NodeMeta{ID: "n"}, Provider: provider, Transport: transport}},
		{"missing Provider", ClusterConfig{Self: NodeMeta{ID: "n", Addr: ":0"}, Transport: transport}},
		{"missing Transport", ClusterConfig{Self: NodeMeta{ID: "n", Addr: ":0"}, Provider: provider}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewCluster(tc.cfg)
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
