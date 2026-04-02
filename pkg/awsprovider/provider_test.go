package awsprovider

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// mockInstanceLister returns canned responses for testing.
type mockInstanceLister struct {
	instances []InstanceInfo
	err       error
}

func (m *mockInstanceLister) ListInstances(_ context.Context, _ map[string]string) ([]InstanceInfo, error) {
	return m.instances, m.err
}

// collectEvents drains up to n events from the channel within a timeout.
func collectEvents(ch <-chan cluster.MemberEvent, n int, timeout time.Duration) []cluster.MemberEvent {
	var events []cluster.MemberEvent
	deadline := time.After(timeout)
	for len(events) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
		case <-deadline:
			return events
		}
	}
	return events
}

func newTestInstances() []InstanceInfo {
	return []InstanceInfo{
		{
			ID:           "i-aaa111",
			PrivateIP:    "10.0.0.1",
			State:        "running",
			Tags:         map[string]string{"env": "prod"},
			Region:       "us-east-1",
			Zone:         "us-east-1a",
			InstanceType: "m5.large",
		},
		{
			ID:           "i-bbb222",
			PrivateIP:    "10.0.0.2",
			State:        "running",
			Tags:         map[string]string{"env": "prod"},
			Region:       "us-east-1",
			Zone:         "us-east-1b",
			InstanceType: "m5.xlarge",
		},
	}
}

func TestAWS_DiscoversMembersFromAPI(t *testing.T) {
	mock := &mockInstanceLister{instances: newTestInstances()}

	p := NewWithConfig(Config{
		Client:       mock,
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 2, 2*time.Second)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	for _, ev := range events {
		if ev.Type != cluster.MemberJoin {
			t.Errorf("expected MemberJoin, got %v", ev.Type)
		}
	}
}

func TestAWS_FiltersNonRunning(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:           "i-running",
			PrivateIP:    "10.0.0.1",
			State:        "running",
			Tags:         map[string]string{},
			Region:       "us-west-2",
			Zone:         "us-west-2a",
			InstanceType: "t3.micro",
		},
		{
			ID:           "i-stopped",
			PrivateIP:    "10.0.0.2",
			State:        "stopped",
			Tags:         map[string]string{},
			Region:       "us-west-2",
			Zone:         "us-west-2a",
			InstanceType: "t3.micro",
		},
	}
	mock := &mockInstanceLister{instances: instances}

	p := NewWithConfig(Config{
		Client:       mock,
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Member.ID != "i-running" {
		t.Errorf("expected i-running, got %s", events[0].Member.ID)
	}
}

func TestAWS_TagMapping(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:           "i-tag1",
			PrivateIP:    "10.0.0.5",
			State:        "running",
			Tags:         map[string]string{"app": "web"},
			Region:       "eu-west-1",
			Zone:         "eu-west-1c",
			InstanceType: "c5.2xlarge",
		},
	}
	mock := &mockInstanceLister{instances: instances}

	p := NewWithConfig(Config{
		Client:       mock,
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	tags := events[0].Member.Tags
	checks := map[string]string{
		"cloud.provider":      "aws",
		"cloud.region":        "eu-west-1",
		"cloud.zone":          "eu-west-1c",
		"cloud.instance-id":   "i-tag1",
		"cloud.instance-type": "c5.2xlarge",
		"app":                 "web",
	}
	for k, want := range checks {
		if got := tags[k]; got != want {
			t.Errorf("tag %q = %q, want %q", k, got, want)
		}
	}
}

func TestAWS_CustomNodeID(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:           "i-custom1",
			PrivateIP:    "10.0.0.1",
			State:        "running",
			Tags:         map[string]string{"name": "worker-1"},
			Region:       "us-east-1",
			Zone:         "us-east-1a",
			InstanceType: "m5.large",
		},
	}
	mock := &mockInstanceLister{instances: instances}

	p := NewWithConfig(Config{
		Client: mock,
		NodeIDFrom: func(info InstanceInfo) cluster.NodeID {
			return cluster.NodeID(info.Tags["name"])
		},
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Member.ID != "worker-1" {
		t.Errorf("expected ID worker-1, got %s", events[0].Member.ID)
	}
}

func TestAWS_CustomAddrFunc(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:           "i-addr1",
			PrivateIP:    "10.0.0.1",
			State:        "running",
			Tags:         map[string]string{},
			Region:       "us-east-1",
			Zone:         "us-east-1a",
			InstanceType: "m5.large",
		},
	}
	mock := &mockInstanceLister{instances: instances}

	p := NewWithConfig(Config{
		Client: mock,
		AddrFunc: func(ip string, info InstanceInfo) string {
			return ip + ":7777"
		},
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Member.Addr != "10.0.0.1:7777" {
		t.Errorf("expected 10.0.0.1:7777, got %s", events[0].Member.Addr)
	}
}

func TestAWS_DefaultPort(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:           "i-port1",
			PrivateIP:    "10.0.0.99",
			State:        "running",
			Tags:         map[string]string{},
			Region:       "us-east-1",
			Zone:         "us-east-1a",
			InstanceType: "t3.nano",
		},
	}
	mock := &mockInstanceLister{instances: instances}

	p := NewWithConfig(Config{
		Client:       mock,
		PollInterval: 50 * time.Millisecond,
	})

	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Member.Addr != "10.0.0.99:9000" {
		t.Errorf("expected 10.0.0.99:9000, got %s", events[0].Member.Addr)
	}
}
