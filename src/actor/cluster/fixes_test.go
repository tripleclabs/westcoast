package cluster

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// --- Fix #2: SetOnMemberEvent is thread-safe ---

func TestFix2_SetOnMemberEvent_Concurrent(t *testing.T) {
	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(context.Background())
	defer c.Stop()

	// Concurrently set the callback and trigger membership events.
	// With the race detector enabled, this would catch the old bug.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			c.SetOnMemberEvent(func(ev MemberEvent) {})
		}(i)
		go func(i int) {
			defer wg.Done()
			provider.AddMember(NodeMeta{
				ID:   NodeID(fmt.Sprintf("node-%d", i+100)),
				Addr: fmt.Sprintf("127.0.0.1:%d", 20000+i),
			})
		}(i)
	}
	wg.Wait()
}

// --- Fix #3: GossipPubSubAdapter buffer cap ---

func TestFix3_GossipPubSubAdapter_BufferFull(t *testing.T) {
	codec := NewGobCodec()
	codec.Register("")

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     codec,
	})

	adapter := NewGossipPubSubAdapter(c, codec, GossipConfig{
		Interval: 1 * time.Hour, // never fires — we want to fill the buffer
		Fanout:   1,
	})

	ctx := context.Background()

	// Fill past the cap.
	var firstErr error
	for i := 0; i < maxPendingPubSub+100; i++ {
		err := adapter.Broadcast(ctx, "topic", "payload", "node-1")
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr == nil {
		t.Error("should return error when buffer is full")
	}
}

// --- Fix #5: FullMeshTopology.Responsible hashes the key ---

func TestFix5_FullMesh_Responsible_UsesKey(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("node-1", "node-2", "node-3")

	// Different keys should map to different responsible nodes
	// (at least some of the time with 3 nodes and many keys).
	responsibleNodes := map[NodeID]bool{}
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key-%d", i)
		resp := topo.Responsible(key, members, 1)
		if len(resp) != 1 {
			t.Fatalf("expected 1 responsible, got %d", len(resp))
		}
		responsibleNodes[resp[0]] = true
	}

	// With 100 keys and 3 nodes, we should see at least 2 different owners.
	if len(responsibleNodes) < 2 {
		t.Errorf("expected multiple responsible nodes across keys, got %d: %v",
			len(responsibleNodes), responsibleNodes)
	}
}

func TestFix5_FullMesh_Responsible_Deterministic(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("node-1", "node-2", "node-3")

	// Same key, same members → same result.
	r1 := topo.Responsible("stable-key", members, 1)
	r2 := topo.Responsible("stable-key", members, 1)
	if r1[0] != r2[0] {
		t.Errorf("should be deterministic: got %s and %s", r1[0], r2[0])
	}
}

func TestFix5_PartitionedRegistry_WithFullMesh(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("node-1", "node-2", "node-3")

	// Two registries on different nodes should agree on the home node.
	r1 := NewPartitionedRegistry("node-1", topo)
	r1.SetMembers(members)
	r2 := NewPartitionedRegistry("node-2", topo)
	r2.SetMembers(members)

	for i := 0; i < 50; i++ {
		name := fmt.Sprintf("svc-%d", i)
		h1 := r1.HomeNode(name)
		h2 := r2.HomeNode(name)
		if h1 != h2 {
			t.Errorf("name %s: node-1 says %s, node-2 says %s", name, h1, h2)
		}
	}
}

// --- Fix #7: ClusterSupervisor phantom decisions ---

func TestFix7_Supervisor_NoPhantomDecisions(t *testing.T) {
	// Create an election where we control when leadership changes.
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	// Force node-1 as leader by removing others if needed.
	scope := clusterSupervisorScope
	leader, _ := election.Leader(scope)
	for leader != "node-1" {
		election.OnMembershipChange(MemberEvent{Type: MemberFailed, Member: NodeMeta{ID: leader}})
		leader, _ = election.Leader(scope)
	}
	// Re-add the nodes so we have live members for placement.
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewGRPCTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(context.Background())
	defer c.Stop()

	registry := NewCRDTRegistry("node-1")
	registry.Register("svc-a", actor.PID{Namespace: "node-3", ActorID: "a", Generation: 1})

	// Use a policy that changes leadership mid-decision.
	termChangingPolicy := &termChangingPolicy{
		election: election,
		inner:    SimpleRestartPolicy{},
	}

	supervisor := NewClusterSupervisor(ClusterSupervisorConfig{
		Election: election,
		Cluster:  c,
		Registry: registry,
		Policy:   termChangingPolicy,
	})
	supervisor.Start(context.Background())
	defer supervisor.Stop()

	provider.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:19999"})
	provider.AddMember(NodeMeta{ID: "node-3", Addr: "127.0.0.1:19998"})
	time.Sleep(50 * time.Millisecond)

	// Trigger node-3 failure.
	c.cfg.OnMemberEvent(MemberEvent{
		Type:   MemberFailed,
		Member: NodeMeta{ID: "node-3"},
	})
	time.Sleep(100 * time.Millisecond)

	// The policy changed the term during OnNodeFailed, so the supervisor
	// should have aborted and recorded zero decisions.
	decisions := supervisor.Decisions()
	if len(decisions) != 0 {
		t.Errorf("expected 0 decisions (leadership changed mid-processing), got %d", len(decisions))
	}
}

// termChangingPolicy changes the election term while computing decisions,
// simulating a leadership change mid-flight.
type termChangingPolicy struct {
	election *RingElection
	inner    ClusterSupervisionPolicy
}

func (p *termChangingPolicy) OnNodeFailed(failedNode NodeID, actorNames []string, liveMembers []NodeMeta) []PlacementDecision {
	decisions := p.inner.OnNodeFailed(failedNode, actorNames, liveMembers)
	// Simulate leadership change by adding a node (which may change the leader).
	p.election.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-99"},
	})
	return decisions
}
