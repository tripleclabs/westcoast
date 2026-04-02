package gcpprovider

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// mockLister implements InstanceLister for testing.
type mockLister struct {
	instances []InstanceInfo
	err       error
}

func (m *mockLister) ListInstances(_ context.Context, _, _ string, _ map[string]string) ([]InstanceInfo, error) {
	return m.instances, m.err
}

// helper to create a provider, start it, drain a join event, and return it.
func startProvider(t *testing.T, instances []InstanceInfo, cfgFn func(*Config)) *GCPProvider {
	t.Helper()

	cfg := Config{
		Project:          "test-project",
		Zone:             "us-central1-a",
		PollInterval:     50 * time.Millisecond,
		FailureThreshold: 3,
		Client:           &mockLister{instances: instances},
	}
	if cfgFn != nil {
		cfgFn(&cfg)
	}

	p := NewWithConfig(cfg)
	self := cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"}
	if err := p.Start(self); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return p
}

func collectEvents(ch <-chan cluster.MemberEvent, n int, timeout time.Duration) []cluster.MemberEvent {
	var events []cluster.MemberEvent
	deadline := time.After(timeout)
	for range n {
		select {
		case ev := <-ch:
			events = append(events, ev)
		case <-deadline:
			return events
		}
	}
	return events
}

func TestGCP_DiscoversMembersFromAPI(t *testing.T) {
	instances := []InstanceInfo{
		{ID: "1001", Name: "node-a", PrivateIP: "10.0.0.1", Status: "RUNNING", Zone: "us-central1-a", MachineType: "e2-medium", Labels: map[string]string{"role": "worker"}},
		{ID: "1002", Name: "node-b", PrivateIP: "10.0.0.2", Status: "RUNNING", Zone: "us-central1-a", MachineType: "e2-small"},
	}

	p := startProvider(t, instances, nil)
	defer p.Stop()

	events := collectEvents(p.Events(), 2, 2*time.Second)
	if len(events) != 2 {
		t.Fatalf("expected 2 join events, got %d", len(events))
	}

	for _, ev := range events {
		if ev.Type != cluster.MemberJoin {
			t.Errorf("expected MemberJoin, got %s", ev.Type)
		}
	}

	members := p.Members()
	if len(members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(members))
	}
}

func TestGCP_FiltersNonRunning(t *testing.T) {
	instances := []InstanceInfo{
		{ID: "1", Name: "running", PrivateIP: "10.0.0.1", Status: "RUNNING", Zone: "us-central1-a", MachineType: "e2-medium"},
		{ID: "2", Name: "staging", PrivateIP: "10.0.0.2", Status: "STAGING", Zone: "us-central1-a", MachineType: "e2-medium"},
		{ID: "3", Name: "terminated", PrivateIP: "10.0.0.3", Status: "TERMINATED", Zone: "us-central1-a", MachineType: "e2-medium"},
		{ID: "4", Name: "stopping", PrivateIP: "10.0.0.4", Status: "STOPPING", Zone: "us-central1-a", MachineType: "e2-medium"},
	}

	p := startProvider(t, instances, nil)
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 join event (only RUNNING), got %d", len(events))
	}

	if events[0].Member.ID != "running" {
		t.Errorf("expected node ID 'running', got %q", events[0].Member.ID)
	}

	members := p.Members()
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
}

func TestGCP_TagMapping(t *testing.T) {
	instances := []InstanceInfo{
		{
			ID:          "42",
			Name:        "worker-1",
			PrivateIP:   "10.0.0.5",
			Status:      "RUNNING",
			Zone:        "europe-west1-b",
			MachineType: "n1-standard-4",
			Labels:      map[string]string{"env": "prod", "team": "infra"},
		},
	}

	p := startProvider(t, instances, nil)
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	tags := events[0].Member.Tags

	expected := map[string]string{
		"cloud.provider":      "gcp",
		"cloud.zone":          "europe-west1-b",
		"cloud.instance-id":   "42",
		"cloud.instance-type": "n1-standard-4",
		"env":                 "prod",
		"team":                "infra",
	}

	for k, want := range expected {
		got, ok := tags[k]
		if !ok {
			t.Errorf("missing tag %q", k)
			continue
		}
		if got != want {
			t.Errorf("tag %q: got %q, want %q", k, got, want)
		}
	}
}

func TestGCP_CustomNodeID(t *testing.T) {
	instances := []InstanceInfo{
		{ID: "99", Name: "my-node", PrivateIP: "10.0.0.1", Status: "RUNNING", Zone: "us-central1-a", MachineType: "e2-medium"},
	}

	p := startProvider(t, instances, func(cfg *Config) {
		cfg.NodeIDFrom = func(info InstanceInfo) cluster.NodeID {
			return cluster.NodeID(fmt.Sprintf("gce-%s", info.ID))
		}
	})
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Member.ID != "gce-99" {
		t.Errorf("expected node ID 'gce-99', got %q", events[0].Member.ID)
	}
}

func TestGCP_DefaultPort(t *testing.T) {
	instances := []InstanceInfo{
		{ID: "1", Name: "node-a", PrivateIP: "10.0.0.1", Status: "RUNNING", Zone: "us-central1-a", MachineType: "e2-medium"},
	}

	p := startProvider(t, instances, nil)
	defer p.Stop()

	events := collectEvents(p.Events(), 1, 2*time.Second)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	want := "10.0.0.1:9000"
	if events[0].Member.Addr != want {
		t.Errorf("expected addr %q, got %q", want, events[0].Member.Addr)
	}
}
