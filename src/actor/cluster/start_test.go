package cluster

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestStart_SingleNode(t *testing.T) {
	ctx := context.Background()
	rt := actor.NewRuntime(actor.WithNodeID("solo"))

	c, err := Start(ctx, rt, Config{
		Addr:     "127.0.0.1:0",
		Provider: NewFixedProvider(FixedProviderConfig{}),
	})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Stop()

	// Singleton should work.
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}
	c.RegisterSingleton(SingletonSpec{Name: "my-singleton", Handler: handler})

	time.Sleep(200 * time.Millisecond)

	running := c.Singletons().Running()
	if len(running) != 1 || running[0] != "my-singleton" {
		t.Errorf("expected singleton running, got %v", running)
	}

	// Daemon should work.
	c.RegisterDaemon(DaemonSpec{Name: "my-daemon", Handler: handler})

	time.Sleep(100 * time.Millisecond)

	daemons := c.Daemons().Running()
	if len(daemons) != 1 || daemons[0] != "my-daemon" {
		t.Errorf("expected daemon running, got %v", daemons)
	}

	// Registry should work.
	pid, _ := rt.IssuePID("solo", "my-singleton")
	if err := c.Register("test-name", pid); err != nil {
		t.Errorf("Register: %v", err)
	}
	got, ok := c.Lookup("test-name")
	if !ok || got.ActorID != "my-singleton" {
		t.Errorf("Lookup: got %v, ok=%v", got, ok)
	}
}

func TestStart_TwoNode_SingletonHandoff(t *testing.T) {
	ctx := context.Background()
	const numSingletons = 10

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	// Shared liveness tracker.
	var mu sync.Mutex
	alive := make(map[string]int)
	var violation atomic.Value

	trackStart := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]++
		if alive[name] > 1 {
			violation.CompareAndSwap(nil, fmt.Sprintf("DUPLICATE: %q count=%d", name, alive[name]))
		}
	}
	trackStop := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]--
	}

	// --- Node 1 ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c1, err := Start(ctx, rt1, Config{
		Addr:     "127.0.0.1:0",
		Provider: p1,
	})
	if err != nil {
		t.Fatalf("Start node-1: %v", err)
	}
	defer c1.Stop()
	addr1 := testTransportAddr(c1.cfg.Transport)

	for i := range numSingletons {
		name := fmt.Sprintf("svc-%d", i)
		c1.RegisterSingleton(SingletonSpec{
			Name:    name,
			Handler: handler,
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

	time.Sleep(200 * time.Millisecond)

	if n := len(c1.Singletons().Running()); n != numSingletons {
		t.Fatalf("expected %d singletons on node-1, got %d", numSingletons, n)
	}

	// --- Node 2 (joining — has node-1 as seed) ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 100 * time.Millisecond,
	})

	c2, err := Start(ctx, rt2, Config{
		Addr:     "127.0.0.1:0",
		Provider: p2,
	})
	if err != nil {
		t.Fatalf("Start node-2: %v", err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)

	// Tell node-1 about node-2 for bidirectional connectivity.
	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Register singletons on node-2 — Start() already waited for
	// peer discovery and registry convergence.
	for i := range numSingletons {
		name := fmt.Sprintf("svc-%d", i)
		c2.RegisterSingleton(SingletonSpec{
			Name:    name,
			Handler: handler,
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

	time.Sleep(3 * time.Second)

	// Check: no duplicates ever occurred.
	if v := violation.Load(); v != nil {
		t.Fatalf("at-most-one violated: %s", v.(string))
	}

	// Check: all singletons running somewhere.
	r1 := c1.Singletons().Running()
	r2 := c2.Singletons().Running()
	total := len(r1) + len(r2)
	if total != numSingletons {
		t.Errorf("expected %d total singletons, got %d (node-1=%d, node-2=%d)", numSingletons, total, len(r1), len(r2))
	}

	t.Logf("final: node-1=%d node-2=%d", len(r1), len(r2))
}

func TestStart_TwoNode_RealisticBoot(t *testing.T) {
	// Realistic scenario: both nodes boot independently, register their
	// singletons immediately (before knowing about each other), then
	// discover peers. No manual sequencing. This is what real code does.
	ctx := context.Background()
	const numSingletons = 10

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	var mu sync.Mutex
	alive := make(map[string]int)
	var violation atomic.Value

	trackStart := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]++
		if alive[name] > 1 {
			violation.CompareAndSwap(nil, fmt.Sprintf("DUPLICATE: %q count=%d", name, alive[name]))
		}
	}
	trackStop := func(name string) {
		mu.Lock()
		defer mu.Unlock()
		alive[name]--
	}

	opts := func() []actor.ActorOption {
		return []actor.ActorOption{
			actor.WithStartHook(func(_ context.Context, id string) error {
				trackStart(id)
				return nil
			}),
			actor.WithStopHook(func(_ context.Context, id string) error {
				trackStop(id)
				return nil
			}),
		}
	}

	// --- Node 1: bootstrap node (no seeds) ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c1, err := Start(ctx, rt1, Config{
		Addr:     "127.0.0.1:0",
		Provider: p1,
	})
	if err != nil {
		t.Fatalf("Start node-1: %v", err)
	}
	defer c1.Stop()
	addr1 := testTransportAddr(c1.cfg.Transport)

	// Node-1 registers singletons immediately — it's the bootstrap node.
	for i := range numSingletons {
		c1.RegisterSingleton(SingletonSpec{
			Name: fmt.Sprintf("svc-%d", i), Handler: handler, Options: opts(),
		})
	}

	time.Sleep(100 * time.Millisecond)

	// --- Node 2: joining node (has seeds) ---
	// It knows about node-1 from the start. cluster.Start() will wait
	// for peer discovery before starting singletons.
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 100 * time.Millisecond,
	})

	c2, err := Start(ctx, rt2, Config{
		Addr:     "127.0.0.1:0",
		Provider: p2,
	})
	if err != nil {
		t.Fatalf("Start node-2: %v", err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)

	// Tell node-1 about node-2 so it can connect back.
	// Must happen before registering singletons so discover responses
	// can reach node-2.
	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Wait for bidirectional connectivity before registering singletons.
	deadline := time.After(10 * time.Second)
	for !c1.IsConnected("node-2") || !c2.IsConnected("node-1") {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cluster connection")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Node-2 registers singletons after connectivity is established.
	for i := range numSingletons {
		c2.RegisterSingleton(SingletonSpec{
			Name: fmt.Sprintf("svc-%d", i), Handler: handler, Options: opts(),
		})
	}

	// Let things settle.
	time.Sleep(2 * time.Second)

	// THE CRITICAL CHECK: no singleton should ever have been duplicated.
	if v := violation.Load(); v != nil {
		t.Fatalf("at-most-one violated: %s", v.(string))
	}

	// Every singleton should be running on exactly one node.
	r1 := c1.Singletons().Running()
	r2 := c2.Singletons().Running()

	set1 := map[string]bool{}
	for _, n := range r1 {
		set1[n] = true
	}
	for _, n := range r2 {
		if set1[n] {
			t.Errorf("singleton %q running on BOTH nodes", n)
		}
	}

	total := len(r1) + len(r2)
	if total != numSingletons {
		t.Errorf("expected %d singletons total, got %d (node-1=%d, node-2=%d)",
			numSingletons, total, len(r1), len(r2))
	}

	t.Logf("realistic boot: node-1=%d node-2=%d", len(r1), len(r2))
}

func TestStart_NodeFailure_SingletonMigrates(t *testing.T) {
	// Node-1 runs singletons. Node-1 fails. Node-2 picks them up.
	ctx := context.Background()

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	// --- Node 1 (bootstrap) ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:0", Provider: p1,
	})
	if err != nil {
		t.Fatal(err)
	}
	addr1 := testTransportAddr(c1.cfg.Transport)

	c1.RegisterSingleton(SingletonSpec{Name: "critical-svc", Handler: handler})
	time.Sleep(200 * time.Millisecond)

	if len(c1.Singletons().Running()) != 1 {
		t.Fatal("singleton should be running on node-1")
	}

	// --- Node 2 (joining) ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 200 * time.Millisecond,
		FailureThreshold:  2,
	})
	// Set a probe that actually checks connectivity.
	p2.Probe = func(ctx context.Context, addr string) error {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}

	c2, err := Start(ctx, rt2, Config{
		Addr: "127.0.0.1:0", Provider: p2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)

	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	c2.RegisterSingleton(SingletonSpec{Name: "critical-svc", Handler: handler})

	time.Sleep(500 * time.Millisecond)

	// Singleton should still be on node-1 (sticky).
	if len(c1.Singletons().Running()) != 1 {
		t.Error("singleton should still be on node-1")
	}
	if len(c2.Singletons().Running()) != 0 {
		t.Error("singleton should NOT be on node-2 yet")
	}

	// Kill node-1.
	c1.Stop()

	// Wait for node-2 to detect failure and pick up the singleton.
	deadline := time.After(10 * time.Second)
	for len(c2.Singletons().Running()) == 0 {
		select {
		case <-deadline:
			t.Fatal("singleton did not migrate to node-2 after node-1 failure")
		case <-time.After(100 * time.Millisecond):
		}
	}

	t.Log("singleton migrated to node-2 after node-1 failure")
}

func TestStart_Rebalance_TransfersState(t *testing.T) {
	// Node-1 runs a singleton that accumulates state. After node-2 joins,
	// an explicit Rebalance moves the singleton to node-2 WITH its state.
	ctx := context.Background()

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state.(int) + 1, nil
	}

	// Track what state node-2's singleton starts with.
	var node2InitialState atomic.Value

	// --- Node 1 ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	p1 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:0", Provider: p1,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Stop()
	addr1 := testTransportAddr(c1.cfg.Transport)

	// Use "counter" because it hashes to node-2 in a {node-1, node-2} cluster.
	c1.RegisterSingleton(SingletonSpec{
		Name: "counter", InitialState: 0, Handler: handler,
	})
	time.Sleep(200 * time.Millisecond)

	// Send messages to accumulate state.
	for range 5 {
		rt1.Send(ctx, "counter", "tick")
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	// --- Node 2 ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{
		Seeds:             []NodeMeta{{ID: "node-1", Addr: addr1}},
		HeartbeatInterval: 100 * time.Millisecond,
	})

	c2, err := Start(ctx, rt2, Config{
		Addr: "127.0.0.1:0", Provider: p2,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Stop()
	addr2 := testTransportAddr(c2.cfg.Transport)

	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})

	// Node-2's handler captures state on first message.
	node2Handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		node2InitialState.CompareAndSwap(nil, state)
		return state.(int) + 1, nil
	}
	c2.RegisterSingleton(SingletonSpec{
		Name: "counter", InitialState: 0, Handler: node2Handler,
	})

	time.Sleep(500 * time.Millisecond)

	// Singleton should still be on node-1 (sticky).
	if len(c2.Singletons().Running()) != 0 {
		t.Fatal("singleton should NOT have moved without rebalance")
	}

	// Rebalance.
	c1.RebalanceSingletons()
	c2.RebalanceSingletons()

	time.Sleep(3 * time.Second)

	if len(c2.Singletons().Running()) != 1 {
		t.Fatal("singleton should be on node-2 after rebalance")
	}

	// Send a probe message so the handler runs and captures state.
	rt2.Send(ctx, "counter", "probe")
	time.Sleep(100 * time.Millisecond)

	v := node2InitialState.Load()
	if v == nil {
		t.Fatal("node-2 handler never ran")
	}
	got := v.(int)
	if got == 0 {
		t.Fatal("state NOT transferred: node-2 started with 0 instead of accumulated state")
	}
	if got < 5 {
		t.Errorf("expected state >= 5, got %d", got)
	}
	t.Logf("state transferred: node-2 started with state=%d", got)
}

func TestStart_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Missing node ID.
	rt := actor.NewRuntime()
	_, err := Start(ctx, rt, Config{
		Addr:     ":9000",
		Provider: NewFixedProvider(FixedProviderConfig{}),
	})
	if err == nil {
		t.Error("expected error for missing node ID")
	}

	// Missing addr.
	rt2 := actor.NewRuntime(actor.WithNodeID("node-1"))
	_, err = Start(ctx, rt2, Config{
		Provider: NewFixedProvider(FixedProviderConfig{}),
	})
	if err == nil {
		t.Error("expected error for missing addr")
	}

	// Missing provider.
	_, err = Start(ctx, rt2, Config{
		Addr: ":9000",
	})
	if err == nil {
		t.Error("expected error for missing provider")
	}
}
