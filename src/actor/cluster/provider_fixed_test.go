package cluster

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestFixedProvider_JoinOnSuccessfulProbe(t *testing.T) {
	p := NewFixedProvider(FixedProviderConfig{
		Seeds:             []string{"127.0.0.1:9001"},
		HeartbeatInterval: 50 * time.Millisecond,
		FailureThreshold:  3,
	})

	p.Dial = func(ctx context.Context, addr string) (NodeMeta, error) {
		return NodeMeta{ID: "node-2", Addr: addr}, nil
	}

	self := NodeMeta{ID: "node-1", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Stop()

	ev := waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberJoin {
		t.Fatalf("expected join, got %v", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Fatalf("expected node-2, got %s", ev.Member.ID)
	}

	members := p.Members()
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
}

func TestFixedProvider_FailureAfterThreshold(t *testing.T) {
	var mu sync.Mutex
	probeCount := 0

	p := NewFixedProvider(FixedProviderConfig{
		Seeds:             []string{"127.0.0.1:9001"},
		HeartbeatInterval: 30 * time.Millisecond,
		FailureThreshold:  2,
	})

	p.Dial = func(ctx context.Context, addr string) (NodeMeta, error) {
		mu.Lock()
		probeCount++
		count := probeCount
		mu.Unlock()

		if count <= 1 {
			return NodeMeta{ID: "node-2", Addr: addr}, nil
		}
		return NodeMeta{}, context.DeadlineExceeded
	}

	self := NodeMeta{ID: "node-1", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Stop()

	// First event: join.
	ev := waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberJoin {
		t.Fatalf("expected join, got %v", ev.Type)
	}

	// After FailureThreshold consecutive failures: failed event.
	ev = waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberFailed {
		t.Fatalf("expected failed, got %v", ev.Type)
	}

	members := p.Members()
	if len(members) != 0 {
		t.Fatalf("expected 0 live members after failure, got %d", len(members))
	}
}

func TestFixedProvider_IgnoresSelf(t *testing.T) {
	p := NewFixedProvider(FixedProviderConfig{
		Seeds:             []string{"127.0.0.1:9000"},
		HeartbeatInterval: 30 * time.Millisecond,
	})

	p.Dial = func(ctx context.Context, addr string) (NodeMeta, error) {
		return NodeMeta{ID: "node-1", Addr: addr}, nil
	}

	self := NodeMeta{ID: "node-1", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Stop()

	// Should NOT emit a join event for self.
	select {
	case ev := <-p.Events():
		t.Fatalf("unexpected event: %+v", ev)
	case <-time.After(150 * time.Millisecond):
		// good
	}
}

func TestFixedProvider_Rejoin(t *testing.T) {
	var mu sync.Mutex
	probeCount := 0

	p := NewFixedProvider(FixedProviderConfig{
		Seeds:             []string{"127.0.0.1:9001"},
		HeartbeatInterval: 30 * time.Millisecond,
		FailureThreshold:  1,
	})

	p.Dial = func(ctx context.Context, addr string) (NodeMeta, error) {
		mu.Lock()
		probeCount++
		count := probeCount
		mu.Unlock()

		// Succeed, then fail once, then succeed again.
		if count == 2 {
			return NodeMeta{}, context.DeadlineExceeded
		}
		return NodeMeta{ID: "node-2", Addr: addr}, nil
	}

	self := NodeMeta{ID: "node-1", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Stop()

	ev := waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberJoin {
		t.Fatalf("expected join, got %v", ev.Type)
	}

	ev = waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberFailed {
		t.Fatalf("expected failed, got %v", ev.Type)
	}

	// Node comes back.
	ev = waitEvent(t, p.Events(), 500*time.Millisecond)
	if ev.Type != MemberJoin {
		t.Fatalf("expected rejoin, got %v", ev.Type)
	}
}

func TestFixedProvider_DoubleStartReturnsError(t *testing.T) {
	p := NewFixedProvider(FixedProviderConfig{Seeds: []string{}})
	self := NodeMeta{ID: "node-1", Addr: ":0"}
	if err := p.Start(self); err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer p.Stop()

	if err := p.Start(self); err == nil {
		t.Fatal("expected error on double start")
	}
}

func TestFixedProvider_AddMember(t *testing.T) {
	p := NewFixedProvider(FixedProviderConfig{})
	self := NodeMeta{ID: "node-1", Addr: ":0"}
	if err := p.Start(self); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer p.Stop()

	p.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:9001"})

	ev := waitEvent(t, p.Events(), 200*time.Millisecond)
	if ev.Type != MemberJoin {
		t.Fatalf("expected join, got %v", ev.Type)
	}

	if len(p.Members()) != 1 {
		t.Fatalf("expected 1 member, got %d", len(p.Members()))
	}

	// Duplicate add is a no-op.
	p.AddMember(NodeMeta{ID: "node-2", Addr: "127.0.0.1:9001"})
	if len(p.Members()) != 1 {
		t.Fatalf("duplicate add should be no-op")
	}
}

func TestMemberEventType_String(t *testing.T) {
	cases := []struct {
		t    MemberEventType
		want string
	}{
		{MemberJoin, "join"},
		{MemberLeave, "leave"},
		{MemberFailed, "failed"},
		{MemberUpdated, "updated"},
		{MemberEventType(99), "unknown"},
	}
	for _, tc := range cases {
		if got := tc.t.String(); got != tc.want {
			t.Errorf("%d: got %s, want %s", tc.t, got, tc.want)
		}
	}
}

func waitEvent(t *testing.T, ch <-chan MemberEvent, timeout time.Duration) MemberEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timeout waiting for event")
		return MemberEvent{}
	}
}
