package cluster

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMetadata_UpdateTags(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	c.UpdateTags(map[string]string{
		"region":        "us-east-1",
		"instance-type": "c5.4xlarge",
	})

	meta := c.Self()
	if meta.Tags["region"] != "us-east-1" {
		t.Errorf("region: got %s", meta.Tags["region"])
	}
	if meta.Tags["instance-type"] != "c5.4xlarge" {
		t.Errorf("instance-type: got %s", meta.Tags["instance-type"])
	}
}

func TestMetadata_RemoveTag(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	c.UpdateTags(map[string]string{"role": "worker", "gpu": "4"})
	c.RemoveTag("gpu")

	meta := c.Self()
	if meta.Tags["role"] != "worker" {
		t.Error("role should survive")
	}
	if _, ok := meta.Tags["gpu"]; ok {
		t.Error("gpu should be removed")
	}
}

func TestMetadata_SelfReturnsCopy(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	c.UpdateTags(map[string]string{"key": "original"})
	meta := c.Self()
	meta.Tags["key"] = "mutated"

	// Original should be unchanged.
	if c.Self().Tags["key"] != "original" {
		t.Error("Self() should return a defensive copy")
	}
}

func TestMetadata_GossipPropagates(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()

	transport1 := NewTCPTransport("node-1")
	transport2 := NewTCPTransport("node-2")
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

	// Create dispatchers and gossip routers.
	d1 := NewInboundDispatcher(nil, codec)
	d2 := NewInboundDispatcher(nil, codec)
	gr1 := NewGossipRouter(d1, codec)
	gr2 := NewGossipRouter(d2, codec)

	gossipCfg := GossipConfig{Interval: 50 * time.Millisecond, Fanout: 3}
	mg1 := NewMetadataGossip(c1, codec, gossipCfg)
	mg2 := NewMetadataGossip(c2, codec, gossipCfg)
	gr1.Add(mg1.GossipProtocol())
	gr2.Add(mg2.GossipProtocol())

	c1.SetOnEnvelope(func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) })
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

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

	// Set tags on node-1.
	c1.UpdateTags(map[string]string{"region": "us-west-2", "gpus": "8"})

	// Start gossip.
	mg1.Start(ctx)
	mg2.Start(ctx)
	defer mg1.Stop()
	defer mg2.Stop()

	// Wait for propagation.
	var found bool
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		members := c2.Members()
		for _, m := range members {
			if m.ID == "node-1" && m.Tags["region"] == "us-west-2" && m.Tags["gpus"] == "8" {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		members := c2.Members()
		t.Fatalf("node-2 should see node-1's tags, got: %+v", members)
	}
}

func TestMetadata_MemberUpdatedEvent(t *testing.T) {
	ctx := context.Background()
	codec := NewGobCodec()

	var mu sync.Mutex
	var events []MemberEvent

	transport1 := NewTCPTransport("node-1")
	transport2 := NewTCPTransport("node-2")
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

	c2.SetOnMemberEvent(func(ev MemberEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})

	d1 := NewInboundDispatcher(nil, codec)
	d2 := NewInboundDispatcher(nil, codec)
	gr1 := NewGossipRouter(d1, codec)
	gr2 := NewGossipRouter(d2, codec)

	gossipCfg := GossipConfig{Interval: 50 * time.Millisecond, Fanout: 3}
	mg1 := NewMetadataGossip(c1, codec, gossipCfg)
	mg2 := NewMetadataGossip(c2, codec, gossipCfg)
	gr1.Add(mg1.GossipProtocol())
	gr2.Add(mg2.GossipProtocol())

	c1.SetOnEnvelope(func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) })
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

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

	mg1.Start(ctx)
	mg2.Start(ctx)
	defer mg1.Stop()
	defer mg2.Stop()

	// Wait for initial gossip to settle.
	time.Sleep(200 * time.Millisecond)

	// Now update tags — should trigger MemberUpdated on node-2.
	c1.UpdateTags(map[string]string{"new-tag": "new-value"})

	// Wait for the updated event.
	var gotUpdated bool
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		mu.Lock()
		for _, ev := range events {
			if ev.Type == MemberUpdated && ev.Member.ID == "node-1" && ev.Member.Tags["new-tag"] == "new-value" {
				gotUpdated = true
			}
		}
		mu.Unlock()
		if gotUpdated {
			break
		}
	}

	if !gotUpdated {
		mu.Lock()
		t.Fatalf("expected MemberUpdated event, got: %+v", events)
		mu.Unlock()
	}
}

func TestMetadata_SingleNodeWorks(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("solo")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "solo", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
	})

	c.UpdateTags(map[string]string{"role": "standalone"})

	meta := c.Self()
	if meta.Tags["role"] != "standalone" {
		t.Errorf("got %s", meta.Tags["role"])
	}
	if meta.ID != "solo" {
		t.Errorf("got %s", meta.ID)
	}
}
