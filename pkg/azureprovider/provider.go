package azureprovider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tripleclabs/westcoast/pkg/providerutil"
	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// Compile-time interface compliance check.
var _ cluster.ClusterProvider = (*AzureProvider)(nil)

// Config controls the Azure VM discovery provider.
type Config struct {
	// SubscriptionID is the Azure subscription to query.
	SubscriptionID string

	// ResourceGroup is the Azure resource group to list VMs from.
	ResourceGroup string

	// TagFilters limits discovery to VMs whose Azure tags contain all
	// of the specified key/value pairs.
	TagFilters map[string]string

	// Port is the cluster transport port advertised in NodeMeta.Addr.
	// Default: 9000.
	Port int

	// AddrFunc optionally overrides address construction.
	// When nil the provider uses "ip:Port".
	AddrFunc func(ip string, info VMInfo) string

	// NodeIDFrom optionally overrides how a cluster.NodeID is derived
	// from a VM. Default: VM name.
	NodeIDFrom func(info VMInfo) cluster.NodeID

	// PollInterval controls how often Azure is polled. Default: 10s.
	PollInterval time.Duration

	// FailureThreshold is the number of consecutive polls a previously
	// seen node must be absent before it is marked failed. Default: 3.
	FailureThreshold int

	// Client is the VMLister implementation. If nil, NewClient is called
	// using SubscriptionID at Start time.
	Client VMLister

	// Probe optionally wraps the discover function with a health probe.
	Probe providerutil.ProbeFunc
}

func (c *Config) setDefaults() {
	if c.Port <= 0 {
		c.Port = 9000
	}
	if c.PollInterval <= 0 {
		c.PollInterval = 10 * time.Second
	}
	if c.FailureThreshold <= 0 {
		c.FailureThreshold = 3
	}
	if c.AddrFunc == nil {
		port := c.Port
		c.AddrFunc = func(ip string, _ VMInfo) string {
			return fmt.Sprintf("%s:%d", ip, port)
		}
	}
	if c.NodeIDFrom == nil {
		c.NodeIDFrom = func(info VMInfo) cluster.NodeID {
			return cluster.NodeID(info.Name)
		}
	}
}

// AzureProvider discovers cluster members by polling Azure VMs.
type AzureProvider struct {
	cfg    Config
	poller *providerutil.Poller
}

// New creates an AzureProvider with sensible defaults.
func New(subscriptionID, resourceGroup string) *AzureProvider {
	return NewWithConfig(Config{
		SubscriptionID: subscriptionID,
		ResourceGroup:  resourceGroup,
	})
}

// NewWithConfig creates an AzureProvider with full control over behaviour.
func NewWithConfig(cfg Config) *AzureProvider {
	cfg.setDefaults()
	return &AzureProvider{cfg: cfg}
}

// Start initialises the Azure client (if needed) and begins polling.
func (a *AzureProvider) Start(self cluster.NodeMeta) error {
	if a.cfg.Client == nil {
		client, err := NewClient(a.cfg.SubscriptionID)
		if err != nil {
			return fmt.Errorf("azureprovider: start: %w", err)
		}
		a.cfg.Client = client
	}

	discover := a.discoverFunc()
	if a.cfg.Probe != nil {
		discover = providerutil.WithProbe(discover, a.cfg.Probe)
	}

	a.poller = providerutil.NewPoller(discover, providerutil.PollerConfig{
		Interval:         a.cfg.PollInterval,
		FailureThreshold: a.cfg.FailureThreshold,
	})

	return a.poller.Start(self)
}

// Stop halts polling.
func (a *AzureProvider) Stop() error {
	if a.poller == nil {
		return cluster.ErrProviderNotStarted
	}
	return a.poller.Stop()
}

// Members returns the current set of alive peers.
func (a *AzureProvider) Members() []cluster.NodeMeta {
	if a.poller == nil {
		return nil
	}
	return a.poller.Members()
}

// Events returns the membership event channel.
func (a *AzureProvider) Events() <-chan cluster.MemberEvent {
	if a.poller == nil {
		return nil
	}
	return a.poller.Events()
}

// discoverFunc builds the DiscoverFunc that calls the Azure API and maps
// VMInfo results to cluster.NodeMeta.
func (a *AzureProvider) discoverFunc() providerutil.DiscoverFunc {
	return func(ctx context.Context) ([]cluster.NodeMeta, error) {
		vms, err := a.cfg.Client.ListVMs(ctx, a.cfg.ResourceGroup, a.cfg.TagFilters)
		if err != nil {
			return nil, err
		}

		var members []cluster.NodeMeta
		for _, vm := range vms {
			// Only include running VMs.
			if !strings.Contains(strings.ToLower(vm.State), "running") {
				continue
			}

			if vm.PrivateIP == "" {
				continue
			}

			tags := make(map[string]string, len(vm.Tags)+4)
			// User tags first so cloud.* keys always win.
			for k, v := range vm.Tags {
				tags[k] = v
			}
			tags["cloud.provider"] = "azure"
			if vm.Location != "" {
				tags["cloud.region"] = vm.Location
			}
			if vm.ID != "" {
				tags["cloud.instance-id"] = vm.ID
			}
			if vm.VMSize != "" {
				tags["cloud.instance-type"] = vm.VMSize
			}

			members = append(members, cluster.NodeMeta{
				ID:       a.cfg.NodeIDFrom(vm),
				Addr:     a.cfg.AddrFunc(vm.PrivateIP, vm),
				Tags:     tags,
				JoinedAt: time.Now(),
			})
		}

		return members, nil
	}
}
