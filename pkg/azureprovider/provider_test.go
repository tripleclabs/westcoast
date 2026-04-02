package azureprovider

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// mockVMLister is a test double for VMLister.
type mockVMLister struct {
	vms []VMInfo
	err error
}

func (m *mockVMLister) ListVMs(_ context.Context, _ string, _ map[string]string) ([]VMInfo, error) {
	return m.vms, m.err
}

func newTestProvider(t *testing.T, vms []VMInfo) *AzureProvider {
	t.Helper()
	p := NewWithConfig(Config{
		SubscriptionID: "sub-1",
		ResourceGroup:  "rg-1",
		PollInterval:   50 * time.Millisecond,
		Client:         &mockVMLister{vms: vms},
	})
	return p
}

func startProvider(t *testing.T, p *AzureProvider) {
	t.Helper()
	err := p.Start(cluster.NodeMeta{ID: "self", Addr: "127.0.0.1:9000"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })
}

func waitForMembers(t *testing.T, p *AzureProvider, want int) []cluster.NodeMeta {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		members := p.Members()
		if len(members) == want {
			return members
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %d members, got %d", want, len(p.Members()))
			return nil
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestAzure_DiscoversMembersFromAPI(t *testing.T) {
	vms := []VMInfo{
		{
			Name:      "node-1",
			ID:        "/subscriptions/sub-1/vms/node-1",
			PrivateIP: "10.0.0.1",
			State:     "PowerState/running",
			Location:  "eastus",
			VMSize:    "Standard_D2s_v3",
		},
		{
			Name:      "node-2",
			ID:        "/subscriptions/sub-1/vms/node-2",
			PrivateIP: "10.0.0.2",
			State:     "PowerState/running",
			Location:  "eastus",
			VMSize:    "Standard_D4s_v3",
		},
	}

	p := newTestProvider(t, vms)
	startProvider(t, p)

	members := waitForMembers(t, p, 2)

	byID := make(map[cluster.NodeID]cluster.NodeMeta, len(members))
	for _, m := range members {
		byID[m.ID] = m
	}

	m1, ok := byID["node-1"]
	if !ok {
		t.Fatal("node-1 not found in members")
	}
	if m1.Addr != "10.0.0.1:9000" {
		t.Errorf("node-1 addr = %q, want %q", m1.Addr, "10.0.0.1:9000")
	}

	m2, ok := byID["node-2"]
	if !ok {
		t.Fatal("node-2 not found in members")
	}
	if m2.Addr != "10.0.0.2:9000" {
		t.Errorf("node-2 addr = %q, want %q", m2.Addr, "10.0.0.2:9000")
	}
}

func TestAzure_FiltersNonRunning(t *testing.T) {
	vms := []VMInfo{
		{
			Name:      "running-vm",
			ID:        "/subscriptions/sub-1/vms/running-vm",
			PrivateIP: "10.0.0.1",
			State:     "PowerState/running",
			Location:  "westus",
		},
		{
			Name:      "stopped-vm",
			ID:        "/subscriptions/sub-1/vms/stopped-vm",
			PrivateIP: "10.0.0.2",
			State:     "PowerState/deallocated",
			Location:  "westus",
		},
		{
			Name:      "starting-vm",
			ID:        "/subscriptions/sub-1/vms/starting-vm",
			PrivateIP: "10.0.0.3",
			State:     "PowerState/starting",
			Location:  "westus",
		},
	}

	p := newTestProvider(t, vms)
	startProvider(t, p)

	members := waitForMembers(t, p, 1)

	if members[0].ID != "running-vm" {
		t.Errorf("expected running-vm, got %q", members[0].ID)
	}
}

func TestAzure_TagMapping(t *testing.T) {
	vms := []VMInfo{
		{
			Name:      "tagged-vm",
			ID:        "/subscriptions/sub-1/vms/tagged-vm",
			PrivateIP: "10.0.0.5",
			State:     "PowerState/running",
			Location:  "northeurope",
			VMSize:    "Standard_B2s",
			Tags:      map[string]string{"env": "prod", "team": "platform"},
		},
	}

	p := newTestProvider(t, vms)
	startProvider(t, p)

	members := waitForMembers(t, p, 1)
	m := members[0]

	checks := map[string]string{
		"cloud.provider":      "azure",
		"cloud.region":        "northeurope",
		"cloud.instance-id":   "/subscriptions/sub-1/vms/tagged-vm",
		"cloud.instance-type": "Standard_B2s",
		"env":                 "prod",
		"team":                "platform",
	}

	for k, want := range checks {
		if got := m.Tags[k]; got != want {
			t.Errorf("tag %q = %q, want %q", k, got, want)
		}
	}
}

func TestAzure_CustomNodeID(t *testing.T) {
	vms := []VMInfo{
		{
			Name:      "vm-abc",
			ID:        "/subscriptions/sub-1/vms/vm-abc",
			PrivateIP: "10.0.0.1",
			State:     "PowerState/running",
		},
	}

	p := NewWithConfig(Config{
		SubscriptionID: "sub-1",
		ResourceGroup:  "rg-1",
		PollInterval:   50 * time.Millisecond,
		Client:         &mockVMLister{vms: vms},
		NodeIDFrom: func(info VMInfo) cluster.NodeID {
			return cluster.NodeID(info.ID)
		},
	})
	startProvider(t, p)

	members := waitForMembers(t, p, 1)
	if members[0].ID != cluster.NodeID("/subscriptions/sub-1/vms/vm-abc") {
		t.Errorf("node ID = %q, want VM resource ID", members[0].ID)
	}
}

func TestAzure_DefaultPort(t *testing.T) {
	vms := []VMInfo{
		{
			Name:      "port-vm",
			PrivateIP: "10.0.0.99",
			State:     "PowerState/running",
		},
	}

	p := newTestProvider(t, vms)
	startProvider(t, p)

	members := waitForMembers(t, p, 1)
	if members[0].Addr != "10.0.0.99:9000" {
		t.Errorf("addr = %q, want %q", members[0].Addr, "10.0.0.99:9000")
	}
}
