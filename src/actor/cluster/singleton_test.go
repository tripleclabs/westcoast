package cluster

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

var _ = fmt.Sprintf  // used in TestSingleton_DistributesAcrossNodes
var _ atomic.Int32   // used in TestSingleton_StartsOnLeader
var _ atomic.Int64   // used in TestSingleton_TwoNodes_NoFlapping

func TestSingleton_StartsOnLeader(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	var called atomic.Int32
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		called.Add(1)
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "my-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/my-singleton")
	running := sm.Running()

	if leader == "node-1" {
		if len(running) != 1 || running[0] != "my-singleton" {
			t.Errorf("node-1 is leader but singleton not running: %v", running)
		}
	} else {
		if len(running) != 0 {
			t.Errorf("node-1 is not leader but singleton is running: %v", running)
		}
	}
}

func TestSingleton_StopsOnLeadershipLoss(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}}) // sole node = leader for everything

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "leader-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// Should be running — we're the only node.
	if len(sm.Running()) != 1 {
		t.Fatal("singleton should be running on sole node")
	}

	// Another node joins — leadership may change.
	election.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/leader-singleton")
	running := sm.Running()

	if leader != "node-1" && len(running) > 0 {
		t.Error("leadership moved but singleton still running")
	}
	if leader == "node-1" && len(running) != 1 {
		t.Error("still leader but singleton not running")
	}
}

func TestSingleton_DistributesAcrossNodes(t *testing.T) {
	members := []NodeMeta{{ID: "node-1"}, {ID: "node-2"}, {ID: "node-3"}}

	// Create elections for each node — same membership, different local IDs.
	elections := make([]*RingElection, 3)
	for i, m := range members {
		elections[i] = NewRingElection(m.ID)
		elections[i].SetMembers(members)
	}

	// All three nodes should agree on leaders, and singletons should spread.
	leaders := map[NodeID]int{}
	for i := 0; i < 20; i++ {
		scope := fmt.Sprintf("singleton/svc-%d", i)
		l1, _ := elections[0].Leader(scope)
		l2, _ := elections[1].Leader(scope)
		l3, _ := elections[2].Leader(scope)

		if l1 != l2 || l2 != l3 {
			t.Errorf("scope %s: nodes disagree on leader: %s %s %s", scope, l1, l2, l3)
		}
		leaders[l1]++
	}

	// With 20 singletons and 3 nodes, should see at least 2 nodes leading.
	if len(leaders) < 2 {
		t.Errorf("singletons not distributed: %v", leaders)
	}
	t.Logf("20 singletons across 3 nodes: %v", leaders)
}

func TestSingleton_MultipleSingletons(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "singleton-a", Handler: handler})
	sm.Register(SingletonSpec{Name: "singleton-b", Handler: handler})
	sm.Register(SingletonSpec{Name: "singleton-c", Handler: handler})

	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	// Sole node — all should be running.
	running := sm.Running()
	if len(running) != 3 {
		t.Errorf("expected 3 running singletons, got %d: %v", len(running), running)
	}
}

func TestSingleton_StopCleansUp(t *testing.T) {
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "cleanup-test", Handler: handler})
	sm.Start(context.Background())

	time.Sleep(100 * time.Millisecond)
	if len(sm.Running()) != 1 {
		t.Fatal("should be running")
	}

	sm.Stop()

	if len(sm.Running()) != 0 {
		t.Error("should have stopped all singletons")
	}

	// Actor should be stopped in the runtime too.
	status := rt.Status("cleanup-test")
	if status != actor.ActorStopped {
		t.Errorf("actor should be stopped, got %s", status)
	}
}

func TestSingleton_ClusterOfOne(t *testing.T) {
	// Single node: no handoff protocol needed, singleton starts immediately.
	election := NewRingElection("solo")
	election.SetMembers([]NodeMeta{{ID: "solo"}})

	rt := actor.NewRuntime(actor.WithNodeID("solo"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "solo-singleton",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(50 * time.Millisecond)

	running := sm.Running()
	if len(running) != 1 || running[0] != "solo-singleton" {
		t.Errorf("expected solo-singleton running, got %v", running)
	}
}

func TestSingleton_MemberFailed_SkipsHandoff(t *testing.T) {
	// When the previous leader fails, the new leader should start immediately
	// without attempting a handoff.
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "failover-test",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/failover-test")
	if leader != "node-1" {
		// If node-1 isn't leader, simulate node-2 failing so node-1 becomes leader.
		sm.OnMemberEvent(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		election.OnMembershipChange(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		time.Sleep(100 * time.Millisecond)
	}

	running := sm.Running()
	if len(running) != 1 {
		t.Errorf("expected singleton running after failover, got %v", running)
	}
}

func TestSingleton_RemoveActorReuse(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	// Create and stop an actor.
	_, err := rt.CreateActor("reuse-test", nil, handler)
	if err != nil {
		t.Fatal(err)
	}
	rt.Stop("reuse-test")

	// Without RemoveActor, CreateActor would fail with duplicate ID.
	err = rt.RemoveActor("reuse-test")
	if err != nil {
		t.Fatalf("RemoveActor failed: %v", err)
	}

	// Now we can reuse the ID.
	_, err = rt.CreateActor("reuse-test", "new-state", handler)
	if err != nil {
		t.Fatalf("CreateActor after RemoveActor failed: %v", err)
	}

	status := rt.Status("reuse-test")
	if status != actor.ActorRunning && status != actor.ActorStarting {
		t.Errorf("expected running/starting, got %s", status)
	}
}

func TestSingleton_RemoveActorStillRunning(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	_, err := rt.CreateActor("running-test", nil, handler)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(50 * time.Millisecond)

	// Should fail — actor is still running.
	err = rt.RemoveActor("running-test")
	if err == nil {
		t.Fatal("expected error removing running actor")
	}
}

func TestSingleton_LeadershipBounce_RestartsSameNode(t *testing.T) {
	// Singleton starts on node-1, leadership moves away and back.
	// The singleton should restart on node-1 (testing RemoveActor path).
	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{
		Name:    "bounce-test",
		Handler: handler,
	})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(50 * time.Millisecond)

	if len(sm.Running()) != 1 {
		t.Fatal("should be running initially")
	}

	// Node-2 joins, leadership may move.
	election.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})
	time.Sleep(100 * time.Millisecond)

	leader, _ := election.Leader("singleton/bounce-test")
	if leader != "node-1" {
		// Leadership moved to node-2. Now node-2 fails, leadership returns.
		sm.OnMemberEvent(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		election.OnMembershipChange(MemberEvent{
			Type:   MemberFailed,
			Member: NodeMeta{ID: "node-2"},
		})
		time.Sleep(100 * time.Millisecond)

		running := sm.Running()
		if len(running) != 1 {
			t.Errorf("expected singleton to restart on node-1 after bounce, got %v", running)
		}
	}
}

func TestSingleton_TwoNodes_NoFlapping(t *testing.T) {
	// Scenario: node-1 boots and starts the singleton. node-2 joins later.
	// At no point should both nodes run the singleton simultaneously, and
	// node-2 joining must not cause unnecessary stop/restart flapping on
	// node-1 if node-1 remains the leader.

	members1 := []NodeMeta{{ID: "node-1"}}

	// Node-1 boots alone — it becomes leader for everything.
	e1 := NewRingElection("node-1")
	e1.SetMembers(members1)

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))

	var starts1 atomic.Int64
	var stops1 atomic.Int64
	handler1 := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm1 := NewSingletonManager(rt1, e1, nil, nil, nil)
	sm1.Register(SingletonSpec{
		Name:         "stable-singleton",
		Handler:      handler1,
		InitialState: "from-node-1",
		Options: []actor.ActorOption{
			actor.WithStartHook(func(_ context.Context, _ string) error {
				starts1.Add(1)
				return nil
			}),
			actor.WithStopHook(func(_ context.Context, _ string) error {
				stops1.Add(1)
				return nil
			}),
		},
	})
	sm1.Start(context.Background())
	defer sm1.Stop()

	time.Sleep(100 * time.Millisecond)

	// Verify node-1 is running the singleton.
	if len(sm1.Running()) != 1 {
		t.Fatal("node-1 should be running the singleton as sole node")
	}
	if starts1.Load() != 1 {
		t.Fatalf("expected exactly 1 start on node-1, got %d", starts1.Load())
	}

	// Node-2 boots with knowledge of both nodes.
	e2 := NewRingElection("node-2")
	e2.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))

	var starts2 atomic.Int64
	handler2 := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm2 := NewSingletonManager(rt2, e2, nil, nil, nil)
	sm2.Register(SingletonSpec{
		Name:    "stable-singleton",
		Handler: handler2,
		Options: []actor.ActorOption{
			actor.WithStartHook(func(_ context.Context, _ string) error {
				starts2.Add(1)
				return nil
			}),
		},
	})
	sm2.Start(context.Background())
	defer sm2.Stop()

	// Now tell node-1's election about node-2 joining.
	e1.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-2"},
	})

	time.Sleep(200 * time.Millisecond)

	// Determine who the leader is across both elections — they must agree.
	l1, _ := e1.Leader("singleton/stable-singleton")
	l2, _ := e2.Leader("singleton/stable-singleton")
	if l1 != l2 {
		t.Fatalf("elections disagree: e1=%s e2=%s", l1, l2)
	}

	running1 := sm1.Running()
	running2 := sm2.Running()

	// CRITICAL: exactly one node should be running the singleton.
	total := len(running1) + len(running2)
	if total != 1 {
		t.Fatalf("expected exactly 1 instance across cluster, got node-1=%v node-2=%v", running1, running2)
	}

	// Verify the correct node is running it.
	if l1 == "node-1" {
		if len(running1) != 1 {
			t.Errorf("node-1 is leader but not running: node-1=%v node-2=%v", running1, running2)
		}
		if len(running2) != 0 {
			t.Errorf("node-2 is not leader but running: %v", running2)
		}
		// If node-1 kept leadership, it should NOT have been restarted.
		if starts1.Load() != 1 {
			t.Errorf("node-1 kept leadership but singleton was restarted: starts=%d stops=%d", starts1.Load(), stops1.Load())
		}
		if starts2.Load() != 0 {
			t.Errorf("node-2 is not leader but started singleton %d times", starts2.Load())
		}
	} else {
		// Leadership moved to node-2.
		if len(running2) != 1 {
			t.Errorf("node-2 is leader but not running: node-1=%v node-2=%v", running1, running2)
		}
		if len(running1) != 0 {
			t.Errorf("node-1 lost leadership but still running: %v", running1)
		}
	}

	// Extra stability check: wait a bit longer and verify nothing flaps.
	time.Sleep(200 * time.Millisecond)

	running1After := sm1.Running()
	running2After := sm2.Running()
	totalAfter := len(running1After) + len(running2After)
	if totalAfter != 1 {
		t.Fatalf("flapping detected: after settling, node-1=%v node-2=%v", running1After, running2After)
	}

	// Start counts should not have changed (no flapping).
	if l1 == "node-1" {
		if starts1.Load() != 1 {
			t.Errorf("flapping on node-1: starts=%d (expected 1)", starts1.Load())
		}
	}
}

func TestSingleton_TwoNodes_JoiningNodeDoesNotDuplicate(t *testing.T) {
	// Regression test: node-1 runs many singletons. node-2 joins and becomes
	// leader for SOME of them (hash distribution). At NO POINT during the
	// entire transition should two instances of the same singleton exist.
	//
	// We prove this with a shared liveness tracker: start hooks increment a
	// per-singleton counter, stop hooks decrement it. If any counter ever
	// exceeds 1, the test fails immediately — not after convergence.
	ctx := context.Background()
	const numSingletons = 20

	// --- Shared liveness tracker across both nodes ---
	var mu sync.Mutex
	alive := make(map[string]int)        // singleton name → current instance count
	var duplicateViolation atomic.Value   // stores first violation as string

	trackStart := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]++
		if alive[name] > 1 {
			msg := fmt.Sprintf("DUPLICATE at start: %q has %d instances", name, alive[name])
			duplicateViolation.CompareAndSwap(nil, msg)
		}
	}
	trackStop := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]--
	}

	makeHandler := func() actor.Handler {
		return func(_ context.Context, state any, msg actor.Message) (any, error) {
			return state, nil
		}
	}

	codec := NewGobCodec()

	// --- Node 1 setup ---
	provider1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	transport1 := NewTCPTransport("node-1")
	c1, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      NoopAuth{},
		Codec:     codec,
	})
	c1.Start(ctx)
	defer c1.Stop()
	addr1 := transport1.listener.Addr().String()

	e1 := NewRingElection("node-1")
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	d1 := NewInboundDispatcher(rt1, codec)
	d1.SetCluster(c1)
	c1.SetOnEnvelope(func(from NodeID, env Envelope) { d1.Dispatch(ctx, from, env) })

	c1.SetDispatcher(d1)
	sm1 := NewSingletonManager(rt1, e1, nil, c1, codec)
	c1.SetOnMemberEvent(func(ev MemberEvent) {
		sm1.OnMemberEvent(ev)
		e1.OnMembershipChange(ev)
	})

	for i := range numSingletons {
		name := fmt.Sprintf("svc-%d", i)
		sm1.Register(SingletonSpec{
			Name:    name,
			Handler: makeHandler(),
			Options: []actor.ActorOption{
				actor.WithStartHook(func(_ context.Context, id string) error {
					trackStart(id)
					return nil
				}),
				actor.WithStopHook(func(_ context.Context, id string) error {
					trackStop(id)
					return nil
				}),
			},
		})
	}
	sm1.Start(ctx)
	defer sm1.Stop()

	time.Sleep(100 * time.Millisecond)

	if len(sm1.Running()) != numSingletons {
		t.Fatalf("expected %d singletons on node-1, got %d", numSingletons, len(sm1.Running()))
	}

	// Figure out which singletons will move to node-2.
	preview := NewRingElection("node-1")
	preview.SetMembers([]NodeMeta{{ID: "node-1"}, {ID: "node-2"}})
	var willMove []string
	for i := range numSingletons {
		name := fmt.Sprintf("svc-%d", i)
		if l, _ := preview.Leader("singleton/" + name); l == "node-2" {
			willMove = append(willMove, name)
		}
	}
	if len(willMove) == 0 {
		t.Skip("no singletons hash to node-2")
	}
	t.Logf("%d of %d singletons will move to node-2: %v", len(willMove), numSingletons, willMove)

	// --- Node 2 setup ---
	provider2 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})
	transport2 := NewTCPTransport("node-2")
	c2, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      NoopAuth{},
		Codec:     codec,
	})
	c2.Start(ctx)
	defer c2.Stop()
	addr2 := transport2.listener.Addr().String()

	e2 := NewRingElection("node-2")
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	d2 := NewInboundDispatcher(rt2, codec)
	d2.SetCluster(c2)
	c2.SetOnEnvelope(func(from NodeID, env Envelope) { d2.Dispatch(ctx, from, env) })

	// Connect the clusters BEFORE starting the singleton manager on node-2.
	provider1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(15 * time.Second)
	for !c1.IsConnected("node-2") || !c2.IsConnected("node-1") {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cluster connection")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Start node-2's singleton manager. It knows about node-1 already.
	c2.SetDispatcher(d2)
	sm2 := NewSingletonManager(rt2, e2, nil, c2, codec)
	c2.SetOnMemberEvent(func(ev MemberEvent) {
		sm2.OnMemberEvent(ev)
		e2.OnMembershipChange(ev)
	})
	e2.OnMembershipChange(MemberEvent{
		Type:   MemberJoin,
		Member: NodeMeta{ID: "node-1"},
	})

	for i := range numSingletons {
		name := fmt.Sprintf("svc-%d", i)
		sm2.Register(SingletonSpec{
			Name:    name,
			Handler: makeHandler(),
			Options: []actor.ActorOption{
				actor.WithStartHook(func(_ context.Context, id string) error {
					trackStart(id)
					return nil
				}),
				actor.WithStopHook(func(_ context.Context, id string) error {
					trackStop(id)
					return nil
				}),
			},
		})
	}
	sm2.Start(ctx)
	defer sm2.Stop()

	// Wait for handoff protocol to settle.
	time.Sleep(3 * time.Second)

	// THE CRITICAL ASSERT: check that no duplicate was ever observed,
	// not just that we converged to one.
	if v := duplicateViolation.Load(); v != nil {
		t.Fatalf("at-most-one violated during handoff: %s", v.(string))
	}

	// Also verify final convergence.
	running1 := sm1.Running()
	running2 := sm2.Running()

	total := len(running1) + len(running2)
	if total != numSingletons {
		t.Errorf("expected %d total singletons, got %d (node-1=%d, node-2=%d)",
			numSingletons, total, len(running1), len(running2))
	}

	t.Logf("final: node-1=%d node-2=%d", len(running1), len(running2))
}
