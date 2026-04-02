package multicastprovider

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// randomMulticastAddr returns a multicast group address using a random
// available port to avoid conflicts between parallel test runs.
func randomMulticastAddr(t *testing.T) string {
	t.Helper()
	// Grab an ephemeral port by briefly listening on UDP.
	conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	conn.Close()
	return fmt.Sprintf("239.1.1.1:%d", port)
}

func testConfig(groupAddr string) Config {
	return Config{
		GroupAddr:         groupAddr,
		Interface:         "lo",
		BroadcastInterval: 100 * time.Millisecond,
		DeadTimeout:       500 * time.Millisecond,
		Port:              9000,
	}
}

func drainEvent(t *testing.T, ch <-chan cluster.MemberEvent, timeout time.Duration) cluster.MemberEvent {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(timeout):
		t.Fatal("timed out waiting for event")
		return cluster.MemberEvent{}
	}
}

func expectNoEvent(t *testing.T, ch <-chan cluster.MemberEvent, duration time.Duration) {
	t.Helper()
	select {
	case ev := <-ch:
		t.Fatalf("unexpected event: %+v", ev)
	case <-time.After(duration):
		// Good, no event.
	}
}

func TestMulticast_JoinOnAnnounce(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p1 := NewWithConfig(cfg)
	p2 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{
		ID:   "node-1",
		Addr: "127.0.0.1:9001",
		Tags: map[string]string{"role": "worker"},
	})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	err = p2.Start(cluster.NodeMeta{
		ID:   "node-2",
		Addr: "127.0.0.1:9002",
		Tags: map[string]string{"role": "scheduler"},
	})
	if err != nil {
		t.Fatalf("p2 start: %v", err)
	}
	defer p2.Stop()

	// p1 should see node-2 join.
	ev := drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected MemberJoin, got %v", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Fatalf("expected node-2, got %v", ev.Member.ID)
	}

	// p2 should see node-1 join.
	ev = drainEvent(t, p2.Events(), 3*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected MemberJoin, got %v", ev.Type)
	}
	if ev.Member.ID != "node-1" {
		t.Fatalf("expected node-1, got %v", ev.Member.ID)
	}
}

func TestMulticast_GracefulLeave(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p1 := NewWithConfig(cfg)
	p2 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	err = p2.Start(cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:9002"})
	if err != nil {
		t.Fatalf("p2 start: %v", err)
	}

	// Wait for p1 to see p2 join.
	ev := drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected MemberJoin, got %v", ev.Type)
	}

	// Now stop p2 gracefully.
	if err := p2.Stop(); err != nil {
		t.Fatalf("p2 stop: %v", err)
	}

	// p1 should receive MemberLeave for node-2.
	ev = drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberLeave {
		t.Fatalf("expected MemberLeave, got %v", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Fatalf("expected node-2, got %v", ev.Member.ID)
	}
}

func TestMulticast_DeadTimeout(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)
	cfg.DeadTimeout = 400 * time.Millisecond

	p1 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	// Create a second provider but we'll simulate death by closing its
	// connection directly without sending a leave packet.
	p2 := NewWithConfig(cfg)
	err = p2.Start(cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:9002"})
	if err != nil {
		t.Fatalf("p2 start: %v", err)
	}

	// Wait for p1 to see p2 join.
	ev := drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected MemberJoin, got %v", ev.Type)
	}

	// Kill p2 ungracefully — close the connection and stop channel
	// without sending a leave packet.
	close(p2.stopCh)
	p2.conn.Close()

	// p1 should detect MemberFailed after the dead timeout.
	ev = drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberFailed {
		t.Fatalf("expected MemberFailed, got %v", ev.Type)
	}
	if ev.Member.ID != "node-2" {
		t.Fatalf("expected node-2, got %v", ev.Member.ID)
	}
}

func TestMulticast_ExcludesSelf(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p1 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	// Wait a few broadcast intervals — should not get any events for self.
	expectNoEvent(t, p1.Events(), 500*time.Millisecond)
}

func TestMulticast_Members(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p1 := NewWithConfig(cfg)
	p2 := NewWithConfig(cfg)
	p3 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	err = p2.Start(cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:9002"})
	if err != nil {
		t.Fatalf("p2 start: %v", err)
	}
	defer p2.Stop()

	err = p3.Start(cluster.NodeMeta{ID: "node-3", Addr: "127.0.0.1:9003"})
	if err != nil {
		t.Fatalf("p3 start: %v", err)
	}
	defer p3.Stop()

	// Wait for p1 to see both peers join.
	drainEvent(t, p1.Events(), 3*time.Second)
	drainEvent(t, p1.Events(), 3*time.Second)

	members := p1.Members()
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d: %+v", len(members), members)
	}

	ids := map[cluster.NodeID]bool{}
	for _, m := range members {
		ids[m.ID] = true
	}
	if !ids["node-2"] || !ids["node-3"] {
		t.Fatalf("expected node-2 and node-3, got %v", ids)
	}
}

func TestMulticast_TagUpdate(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p1 := NewWithConfig(cfg)
	p2 := NewWithConfig(cfg)

	err := p1.Start(cluster.NodeMeta{
		ID:   "node-1",
		Addr: "127.0.0.1:9001",
		Tags: map[string]string{"role": "worker"},
	})
	if err != nil {
		t.Fatalf("p1 start: %v", err)
	}
	defer p1.Stop()

	err = p2.Start(cluster.NodeMeta{
		ID:   "node-2",
		Addr: "127.0.0.1:9002",
		Tags: map[string]string{"role": "worker"},
	})
	if err != nil {
		t.Fatalf("p2 start: %v", err)
	}
	defer p2.Stop()

	// Wait for p1 to see p2 join.
	ev := drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberJoin {
		t.Fatalf("expected MemberJoin, got %v", ev.Type)
	}

	// Update p2's tags.
	p2.UpdateTags(map[string]string{"role": "scheduler"})

	// p1 should receive a MemberUpdated event.
	ev = drainEvent(t, p1.Events(), 3*time.Second)
	if ev.Type != cluster.MemberUpdated {
		t.Fatalf("expected MemberUpdated, got %v", ev.Type)
	}
	if ev.Member.Tags["role"] != "scheduler" {
		t.Fatalf("expected role=scheduler, got %v", ev.Member.Tags["role"])
	}
}

func TestMulticast_DoubleStart(t *testing.T) {
	groupAddr := randomMulticastAddr(t)
	cfg := testConfig(groupAddr)

	p := NewWithConfig(cfg)
	err := p.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	defer p.Stop()

	err = p.Start(cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:9001"})
	if err != cluster.ErrProviderAlreadyStarted {
		t.Fatalf("expected ErrProviderAlreadyStarted, got %v", err)
	}
}

func TestMulticast_StopBeforeStart(t *testing.T) {
	p := New()
	err := p.Stop()
	if err != cluster.ErrProviderNotStarted {
		t.Fatalf("expected ErrProviderNotStarted, got %v", err)
	}
}
