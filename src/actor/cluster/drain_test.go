package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestDrain_NotifiesPeers(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var events []MemberEvent

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.SetOnMemberEvent(func(ev MemberEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	})
	c.Start(ctx)

	err := Drain(ctx, c, DrainConfig{Timeout: 1 * time.Second})
	if err != nil {
		t.Fatalf("drain: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	found := false
	for _, ev := range events {
		if ev.Type == MemberLeave && ev.Member.ID == "node-1" {
			found = true
		}
	}
	if !found {
		t.Error("should emit MemberLeave for self during drain")
	}
}

func TestDrain_StopsSingletons(t *testing.T) {
	ctx := context.Background()

	election := NewRingElection("node-1")
	election.SetMembers([]NodeMeta{{ID: "node-1"}})

	rt := actor.NewRuntime(actor.WithNodeID("node-1"))
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "drain-singleton", Handler: handler})
	sm.Start(ctx)

	time.Sleep(100 * time.Millisecond)
	if len(sm.Running()) != 1 {
		t.Fatal("singleton should be running before drain")
	}

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(ctx)

	err := Drain(ctx, c, DrainConfig{Timeout: 2 * time.Second}, WithSingletonManager(sm))
	if err != nil {
		t.Fatalf("drain: %v", err)
	}

	if len(sm.Running()) != 0 {
		t.Error("singletons should be stopped after drain")
	}
}

func TestDrain_DeregistersNames(t *testing.T) {
	ctx := context.Background()

	registry := NewDistributedRegistry("node-1")
	registry.Register("svc-a", actor.PID{Namespace: "node-1", ActorID: "a", Generation: 1})
	registry.Register("svc-b", actor.PID{Namespace: "node-1", ActorID: "b", Generation: 1})
	registry.Register("svc-c", actor.PID{Namespace: "node-2", ActorID: "c", Generation: 1}) // different node

	provider := NewFixedProvider(FixedProviderConfig{})
	transport := NewTCPTransport("node-1")
	c, _ := NewCluster(ClusterConfig{
		Self:      NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider,
		Transport: transport,
		Auth:      NoopAuth{},
		Codec:     NewGobCodec(),
	})
	c.Start(ctx)

	err := Drain(ctx, c, DrainConfig{Timeout: 1 * time.Second}, WithRegistry(registry))
	if err != nil {
		t.Fatalf("drain: %v", err)
	}

	// Names from node-1 should be gone.
	if _, ok := registry.Lookup("svc-a"); ok {
		t.Error("svc-a should be deregistered")
	}
	if _, ok := registry.Lookup("svc-b"); ok {
		t.Error("svc-b should be deregistered")
	}
	// Names from other nodes should survive.
	if _, ok := registry.Lookup("svc-c"); !ok {
		t.Error("svc-c (node-2) should survive drain")
	}
}
