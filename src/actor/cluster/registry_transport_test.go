package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRegistryReplication_TwoNodes(t *testing.T) {
	// Verify that a name registered on node-1 replicates to node-2
	// via the CRDT transport within a reasonable time.
	ctx := context.Background()
	codec := NewGobCodec()

	// Node 1.
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	t1 := newTestTransport("node-1")
	c1, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"}, Provider: p1,
		Transport: t1, Auth: NoopAuth{}, Codec: codec,
	})
	c1.Start(ctx)
	defer c1.Stop()
	addr1 := testTransportAddr(t1)

	d1 := NewInboundDispatcher(&noopBridge{nodeID: "node-1"}, codec)
	d1.SetCluster(c1)
	c1.SetOnEnvelope(func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) })

	reg1 := NewDistributedRegistry("node-1", WithClusterReplication(c1))
	reg1.WireInbound(d1)

	// Node 2.
	p2 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	t2 := newTestTransport("node-2")
	c2, _ := NewCluster(ClusterConfig{
		Self: NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"}, Provider: p2,
		Transport: t2, Auth: NoopAuth{}, Codec: codec,
	})
	c2.Start(ctx)
	defer c2.Stop()
	addr2 := testTransportAddr(t2)

	d2 := NewInboundDispatcher(&noopBridge{nodeID: "node-2"}, codec)
	d2.SetCluster(c2)
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

	reg2 := NewDistributedRegistry("node-2", WithClusterReplication(c2))
	reg2.WireInbound(d2)

	// Connect.
	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	p2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(5 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Register a name on node-1.
	pid := actor.PID{Namespace: "node-1", ActorID: "test-actor", Generation: 1}
	if err := reg1.Register("my-service", pid); err != nil {
		t.Fatal(err)
	}

	// Wait for it to appear on node-2.
	replicateDeadline := time.After(10 * time.Second)
	for {
		if got, ok := reg2.Lookup("my-service"); ok {
			if got.ActorID == "test-actor" {
				t.Logf("replicated in time: %v", got)
				return
			}
		}
		select {
		case <-replicateDeadline:
			entries1 := reg1.AllEntries()
			entries2 := reg2.AllEntries()
			t.Fatalf("registry did not replicate in 10s: node-1=%v node-2=%v", entries1, entries2)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func TestRegistryReplication_ViaClusterStart(t *testing.T) {
	// Same as above but using the high-level cluster.Start() API.
	ctx := context.Background()

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	c1, err := Start(ctx, rt1, Config{Addr: "127.0.0.1:0", Provider: p1})
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Stop()
	addr1 := testTransportAddr(c1.cfg.Transport)

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := Start(ctx, rt2, Config{Addr: "127.0.0.1:0", Provider: p2})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)

	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Wait for connectivity.
	deadline := time.After(5 * time.Second)
	for !c1.IsConnected("node-2") {
		select {
		case <-deadline:
			t.Fatal("timeout connecting")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Register on node-1.
	pid := actor.PID{Namespace: "node-1", ActorID: "test-svc", Generation: 1}
	c1.Register("test-svc", pid)

	// Check node-2's registry.
	regDeadline := time.After(5 * time.Second)
	for {
		if got, ok := c2.Lookup("test-svc"); ok {
			t.Logf("replicated via Start(): %v", got)
			return
		}
		select {
		case <-regDeadline:
			t.Fatal("registry did not replicate via Start() in 5s")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// noopBridge implements RuntimeBridge for tests that don't need actor delivery.
type noopBridge struct {
	nodeID string
}

func (b *noopBridge) DeliverLocal(_ context.Context, _ string, _ any, _ *actor.AskRequestContext) actor.SubmitAck {
	return actor.SubmitAck{}
}
func (b *noopBridge) DeliverPID(_ context.Context, _ actor.PID, _ any) actor.PIDSendAck {
	return actor.PIDSendAck{}
}
func (b *noopBridge) NodeID() string { return b.nodeID }
