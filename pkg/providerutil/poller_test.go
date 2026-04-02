package providerutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

func waitEvent(t *testing.T, ch <-chan cluster.MemberEvent, timeout time.Duration) cluster.MemberEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timeout waiting for event")
		return cluster.MemberEvent{}
	}
}

func noEvent(t *testing.T, ch <-chan cluster.MemberEvent, wait time.Duration) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event: %v", ev)
	case <-time.After(wait):
	}
}

func TestPoller_JoinOnFirstPoll(t *testing.T) {
	discover := staticDiscover([]cluster.NodeMeta{
		{ID: "node-2", Addr: "10.0.0.2:9000"},
	})

	p := NewPoller(discover, PollerConfig{Interval: 50 * time.Millisecond})
	if err := p.Start(cluster.NodeMeta{ID: "node-1"}); err != nil {
		t.Fatal(err)
	}
	defer p.Stop()

	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Errorf("expected MemberJoin, got %s", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Errorf("expected node-2, got %s", ev.Member.ID)
	}
}

func TestPoller_ExcludesSelf(t *testing.T) {
	discover := staticDiscover([]cluster.NodeMeta{
		{ID: "node-1", Addr: "10.0.0.1:9000"}, // self
		{ID: "node-2", Addr: "10.0.0.2:9000"},
	})

	p := NewPoller(discover, PollerConfig{Interval: 50 * time.Millisecond})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Member.ID != "node-2" {
		t.Errorf("expected node-2, got %s", ev.Member.ID)
	}

	// Should not emit a join for self.
	noEvent(t, p.Events(), 200*time.Millisecond)
}

func TestPoller_FailureAfterThreshold(t *testing.T) {
	var mu sync.Mutex
	members := []cluster.NodeMeta{{ID: "node-2", Addr: "10.0.0.2:9000"}}

	discover := func(ctx context.Context) ([]cluster.NodeMeta, error) {
		mu.Lock()
		defer mu.Unlock()
		result := make([]cluster.NodeMeta, len(members))
		copy(result, members)
		return result, nil
	}

	p := NewPoller(discover, PollerConfig{
		Interval:         50 * time.Millisecond,
		FailureThreshold: 2,
	})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	// Wait for join.
	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected join, got %s", ev.Type)
	}

	// Remove the member.
	mu.Lock()
	members = nil
	mu.Unlock()

	// Should fail after 2 misses.
	ev = waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberFailed {
		t.Errorf("expected MemberFailed, got %s", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Errorf("expected node-2, got %s", ev.Member.ID)
	}
}

func TestPoller_Rejoin(t *testing.T) {
	var mu sync.Mutex
	members := []cluster.NodeMeta{{ID: "node-2", Addr: "10.0.0.2:9000"}}

	discover := func(ctx context.Context) ([]cluster.NodeMeta, error) {
		mu.Lock()
		defer mu.Unlock()
		result := make([]cluster.NodeMeta, len(members))
		copy(result, members)
		return result, nil
	}

	p := NewPoller(discover, PollerConfig{
		Interval:         50 * time.Millisecond,
		FailureThreshold: 1, // fail immediately on first miss
	})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	// Join.
	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected join, got %s", ev.Type)
	}

	// Remove → fail.
	mu.Lock()
	members = nil
	mu.Unlock()
	ev = waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberFailed {
		t.Fatalf("expected failed, got %s", ev.Type)
	}

	// Re-add → rejoin.
	mu.Lock()
	members = []cluster.NodeMeta{{ID: "node-2", Addr: "10.0.0.2:9000"}}
	mu.Unlock()
	ev = waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected rejoin, got %s", ev.Type)
	}
}

func TestPoller_TagUpdate(t *testing.T) {
	var mu sync.Mutex
	members := []cluster.NodeMeta{
		{ID: "node-2", Addr: "10.0.0.2:9000", Tags: map[string]string{"zone": "a"}},
	}

	discover := func(ctx context.Context) ([]cluster.NodeMeta, error) {
		mu.Lock()
		defer mu.Unlock()
		result := make([]cluster.NodeMeta, len(members))
		copy(result, members)
		return result, nil
	}

	p := NewPoller(discover, PollerConfig{Interval: 50 * time.Millisecond})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	// Join.
	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected join, got %s", ev.Type)
	}

	// Change tags.
	mu.Lock()
	members = []cluster.NodeMeta{
		{ID: "node-2", Addr: "10.0.0.2:9000", Tags: map[string]string{"zone": "b"}},
	}
	mu.Unlock()

	ev = waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberUpdated {
		t.Errorf("expected MemberUpdated, got %s", ev.Type)
	}
	if ev.Member.Tags["zone"] != "b" {
		t.Errorf("expected zone=b, got %s", ev.Member.Tags["zone"])
	}
}

func TestPoller_Members(t *testing.T) {
	discover := staticDiscover([]cluster.NodeMeta{
		{ID: "node-2", Addr: "10.0.0.2:9000"},
		{ID: "node-3", Addr: "10.0.0.3:9000"},
	})

	p := NewPoller(discover, PollerConfig{Interval: 50 * time.Millisecond})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	// Wait for both joins.
	waitEvent(t, p.Events(), 2*time.Second)
	waitEvent(t, p.Events(), 2*time.Second)

	members := p.Members()
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestPoller_DoubleStartReturnsError(t *testing.T) {
	p := NewPoller(staticDiscover(nil), PollerConfig{})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	if err := p.Start(cluster.NodeMeta{ID: "node-1"}); err != cluster.ErrProviderAlreadyStarted {
		t.Errorf("expected ErrProviderAlreadyStarted, got %v", err)
	}
}

func TestPoller_DiscoveryErrorSkipsCycle(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	discover := func(ctx context.Context) ([]cluster.NodeMeta, error) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()

		if n == 1 {
			// First call: return a member.
			return []cluster.NodeMeta{{ID: "node-2", Addr: "10.0.0.2:9000"}}, nil
		}
		// Second call: error.
		return nil, context.DeadlineExceeded
	}

	p := NewPoller(discover, PollerConfig{
		Interval:         50 * time.Millisecond,
		FailureThreshold: 1,
	})
	p.Start(cluster.NodeMeta{ID: "node-1"})
	defer p.Stop()

	// Should get the join from first poll.
	ev := waitEvent(t, p.Events(), 2*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected join, got %s", ev.Type)
	}

	// Error polls should NOT cause a failure event.
	noEvent(t, p.Events(), 300*time.Millisecond)
}

// staticDiscover returns a DiscoverFunc that always returns the same set.
func staticDiscover(members []cluster.NodeMeta) DiscoverFunc {
	return func(ctx context.Context) ([]cluster.NodeMeta, error) {
		result := make([]cluster.NodeMeta, len(members))
		copy(result, members)
		return result, nil
	}
}
