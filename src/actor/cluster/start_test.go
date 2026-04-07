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
	addr1 := c1.cfg.Transport.(*TCPTransport).listener.Addr().String()

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

	// --- Node 2 ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	p2 := NewFixedProvider(FixedProviderConfig{HeartbeatInterval: 100 * time.Millisecond})

	c2, err := Start(ctx, rt2, Config{
		Addr:     "127.0.0.1:0",
		Provider: p2,
	})
	if err != nil {
		t.Fatalf("Start node-2: %v", err)
	}
	defer c2.Stop()
	addr2 := c2.cfg.Transport.(*TCPTransport).listener.Addr().String()

	// Introduce the nodes and wait for connectivity.
	p1.AddMember(NodeMeta{ID: "node-2", Addr: addr2})
	p2.AddMember(NodeMeta{ID: "node-1", Addr: addr1})

	deadline := time.After(10 * time.Second)
	for !c1.IsConnected("node-2") || !c2.IsConnected("node-1") {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cluster connection")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// Register singletons on node-2 AFTER connectivity is established,
	// so findPrevLeader can see node-1 in the cluster membership.
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
