package cluster

import (
	"fmt"
	"testing"
)

func makeMembers(ids ...string) []NodeMeta {
	out := make([]NodeMeta, len(ids))
	for i, id := range ids {
		out[i] = NodeMeta{ID: NodeID(id), Addr: fmt.Sprintf("10.0.0.%d:9000", i+1)}
	}
	return out
}

// --- FullMeshTopology ---

func TestFullMesh_ShouldConnectAll(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("a", "b", "c", "d")

	targets := topo.ShouldConnect("a", members)
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d: %v", len(targets), targets)
	}

	has := make(map[NodeID]bool)
	for _, id := range targets {
		has[id] = true
	}
	for _, id := range []NodeID{"b", "c", "d"} {
		if !has[id] {
			t.Errorf("missing %s", id)
		}
	}
}

func TestFullMesh_RouteDirect(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("a", "b", "c")

	hop, ok := topo.Route("a", "c", members)
	if !ok {
		t.Fatal("should find route")
	}
	if hop != "c" {
		t.Errorf("expected direct route to c, got %s", hop)
	}
}

func TestFullMesh_RouteUnknownTarget(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("a", "b")

	_, ok := topo.Route("a", "z", members)
	if ok {
		t.Error("should not route to unknown node")
	}
}

func TestFullMesh_Responsible(t *testing.T) {
	topo := FullMeshTopology{}
	members := makeMembers("a", "b", "c")

	r := topo.Responsible("some-key", members, 2)
	if len(r) != 2 {
		t.Fatalf("expected 2 responsible nodes, got %d", len(r))
	}
}

// --- RingTopology ---

func TestRing_ShouldConnect_SmallCluster(t *testing.T) {
	// With 4 nodes and fanout 2, 2*fanout+1 = 5 > 4, so full mesh.
	topo := NewRingTopology(2, 1)
	members := makeMembers("a", "b", "c", "d")

	targets := topo.ShouldConnect("a", members)
	if len(targets) != 3 {
		t.Errorf("small cluster should be full mesh, got %d targets", len(targets))
	}
}

func TestRing_ShouldConnect_LargeCluster(t *testing.T) {
	// With 20 nodes and fanout 2, should connect to ~2*2 + log2(20) ≈ 8-9 nodes.
	topo := NewRingTopology(2, 1)
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = fmt.Sprintf("node-%02d", i)
	}
	members := makeMembers(ids...)

	targets := topo.ShouldConnect("node-00", members)

	// Should be fewer than full mesh (19).
	if len(targets) >= 19 {
		t.Errorf("ring topology should reduce connections, got %d", len(targets))
	}
	// Should have at least 2*fanout neighbors.
	if len(targets) < 4 {
		t.Errorf("should have at least 4 neighbors, got %d", len(targets))
	}

	t.Logf("20-node ring with fanout=2: %d connections", len(targets))
}

func TestRing_RouteDirect_Neighbor(t *testing.T) {
	topo := NewRingTopology(2, 1)
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
	}
	members := makeMembers(ids...)

	// Route to a neighbor — should return direct.
	targets := topo.ShouldConnect("n0", members)
	if len(targets) == 0 {
		t.Fatal("no connections")
	}

	// Route to one of the connected targets should succeed.
	hop, ok := topo.Route("n0", targets[0], members)
	if !ok {
		t.Fatal("should route to neighbor")
	}
	// The hop should either be the target (direct) or an intermediate.
	if hop == "" {
		t.Error("hop should not be empty")
	}
}

func TestRing_RouteMultiHop(t *testing.T) {
	// With fanout=1 and 10 nodes, most destinations require multiple hops.
	topo := NewRingTopology(1, 1)
	ids := make([]string, 10)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%d", i)
	}
	members := makeMembers(ids...)

	// Try routing from n0 to all other nodes.
	for _, id := range ids[1:] {
		hop, ok := topo.Route("n0", NodeID(id), members)
		if !ok {
			t.Errorf("should find route to %s", id)
			continue
		}
		if hop == "" {
			t.Errorf("empty hop for %s", id)
		}
	}
}

func TestRing_RouteConverges(t *testing.T) {
	// Verify that multi-hop routing converges (doesn't loop).
	topo := NewRingTopology(1, 1)
	ids := make([]string, 20)
	for i := range ids {
		ids[i] = fmt.Sprintf("n%02d", i)
	}
	members := makeMembers(ids...)
	target := NodeID("n15")

	current := NodeID("n00")
	visited := map[NodeID]bool{current: true}

	for i := 0; i < 30; i++ { // max 30 hops (should converge in ~log2(20) ≈ 5)
		if current == target {
			t.Logf("reached %s in %d hops", target, i)
			return
		}

		hop, ok := topo.Route(current, target, members)
		if !ok {
			t.Fatalf("no route from %s to %s", current, target)
		}
		if visited[hop] && hop != target {
			t.Fatalf("routing loop: visited %s again at hop %d", hop, i)
		}
		visited[hop] = true
		current = hop
	}
	t.Fatalf("routing did not converge in 30 hops")
}

func TestRing_RouteAroundDeadNode(t *testing.T) {
	topo := NewRingTopology(1, 1)
	allIDs := make([]string, 10)
	for i := range allIDs {
		allIDs[i] = fmt.Sprintf("n%d", i)
	}

	// Remove n5 from the member list (simulating failure).
	var liveIDs []string
	for _, id := range allIDs {
		if id != "n5" {
			liveIDs = append(liveIDs, id)
		}
	}
	members := makeMembers(liveIDs...)

	// Route to n5 should fail (not in membership).
	_, ok := topo.Route("n0", "n5", members)
	if ok {
		t.Error("should not route to dead node")
	}

	// Route to nodes beyond n5 should still work.
	for _, id := range liveIDs[1:] {
		_, ok := topo.Route("n0", NodeID(id), members)
		if !ok {
			t.Errorf("should route to %s around dead n5", id)
		}
	}
}

func TestRing_Responsible(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("a", "b", "c", "d", "e")

	// Replication=3: should return 3 unique nodes.
	resp := topo.Responsible("my-key", members, 3)
	if len(resp) != 3 {
		t.Fatalf("expected 3 responsible nodes, got %d", len(resp))
	}

	// All should be unique.
	seen := make(map[NodeID]bool)
	for _, id := range resp {
		if seen[id] {
			t.Errorf("duplicate node %s", id)
		}
		seen[id] = true
	}
}

func TestRing_Responsible_Stable(t *testing.T) {
	topo := NewRingTopology(2, 10)
	members := makeMembers("a", "b", "c", "d", "e")

	// Same key should always map to the same nodes.
	r1 := topo.Responsible("stable-key", members, 1)
	r2 := topo.Responsible("stable-key", members, 1)
	if r1[0] != r2[0] {
		t.Errorf("expected stable mapping, got %s and %s", r1[0], r2[0])
	}
}

func TestRing_VNodes_DistributeEvenly(t *testing.T) {
	// With more nodes and vnodes, distribution should be reasonably even.
	topo := NewRingTopology(2, 100)
	members := makeMembers("node-a", "node-b", "node-c", "node-d", "node-e")

	counts := make(map[NodeID]int)
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("key-%d", i)
		resp := topo.Responsible(key, members, 1)
		counts[resp[0]]++
	}

	// Each of 5 nodes should get roughly 1000 keys. Allow wide tolerance
	// since consistent hashing has natural variance.
	for id, count := range counts {
		if count < 400 || count > 1800 {
			t.Errorf("node %s got %d keys (expected ~1000)", id, count)
		}
	}
	// Verify all nodes got some keys.
	if len(counts) != 5 {
		t.Errorf("expected all 5 nodes to get keys, got %d nodes", len(counts))
	}
	t.Logf("distribution: %v", counts)
}

func TestRing_ConnectionCount_Scales(t *testing.T) {
	topo := NewRingTopology(2, 1)

	sizes := []int{5, 10, 20, 50, 100}
	for _, size := range sizes {
		ids := make([]string, size)
		for i := range ids {
			ids[i] = fmt.Sprintf("n%03d", i)
		}
		members := makeMembers(ids...)
		targets := topo.ShouldConnect("n000", members)

		t.Logf("cluster size %d: %d connections (full mesh would be %d)",
			size, len(targets), size-1)

		if size > 10 && len(targets) >= size-1 {
			t.Errorf("ring should reduce connections for %d nodes", size)
		}
	}
}
