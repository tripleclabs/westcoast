package gcpprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/tripleclabs/westcoast/pkg/providerutil"
	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// Compile-time interface check.
var _ cluster.ClusterProvider = (*GCPProvider)(nil)

// Config controls the GCP instance discovery behaviour.
type Config struct {
	// Project is the GCP project ID. Required.
	Project string

	// Zone is the GCE zone to query (e.g. "us-central1-a"). Required.
	Zone string

	// LabelFilters restricts discovery to instances that have all of
	// these GCE labels set to the given values.
	LabelFilters map[string]string

	// Port is the transport port advertised in NodeMeta.Addr.
	// Default: 9000.
	Port int

	// AddrFunc overrides how the advertised address is computed from
	// an instance's private IP and info. Default: "ip:Port".
	AddrFunc func(ip string, info InstanceInfo) string

	// NodeIDFrom overrides how the cluster NodeID is derived from an
	// instance. Default: instance name.
	NodeIDFrom func(info InstanceInfo) cluster.NodeID

	// PollInterval is the time between discovery polls. Default: 10s.
	PollInterval time.Duration

	// FailureThreshold is the number of consecutive missed polls before
	// a node is declared failed. Default: 3.
	FailureThreshold int

	// Client is the InstanceLister to use. If nil, a real GCE client is
	// created using Application Default Credentials on Start.
	Client InstanceLister

	// Probe optionally wraps the discover function so each discovered
	// node is health-checked before being reported as alive.
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
}

// GCPProvider discovers cluster members by polling the GCE instances API.
type GCPProvider struct {
	cfg    Config
	poller *providerutil.Poller
}

// New creates a GCPProvider for the given project and zone with default
// settings.
func New(project, zone string) *GCPProvider {
	return NewWithConfig(Config{
		Project: project,
		Zone:    zone,
	})
}

// NewWithConfig creates a GCPProvider with full configuration control.
func NewWithConfig(cfg Config) *GCPProvider {
	cfg.setDefaults()
	return &GCPProvider{cfg: cfg}
}

// Start initialises the GCE client (if needed) and begins polling.
// Implements cluster.ClusterProvider.
func (p *GCPProvider) Start(self cluster.NodeMeta) error {
	if p.cfg.Client == nil {
		c, err := NewClient(context.Background())
		if err != nil {
			return fmt.Errorf("gcpprovider: %w", err)
		}
		p.cfg.Client = c
	}

	discover := p.makeDiscoverFunc()
	if p.cfg.Probe != nil {
		discover = providerutil.WithProbe(discover, p.cfg.Probe)
	}

	p.poller = providerutil.NewPoller(discover, providerutil.PollerConfig{
		Interval:         p.cfg.PollInterval,
		FailureThreshold: p.cfg.FailureThreshold,
	})
	return p.poller.Start(self)
}

// Stop halts polling and closes the event channel.
// Implements cluster.ClusterProvider.
func (p *GCPProvider) Stop() error {
	return p.poller.Stop()
}

// Members returns the current set of alive peers.
// Implements cluster.ClusterProvider.
func (p *GCPProvider) Members() []cluster.NodeMeta {
	return p.poller.Members()
}

// Events returns the membership event channel.
// Implements cluster.ClusterProvider.
func (p *GCPProvider) Events() <-chan cluster.MemberEvent {
	return p.poller.Events()
}

// makeDiscoverFunc returns a DiscoverFunc that queries the GCE API and
// maps instances to NodeMeta values.
func (p *GCPProvider) makeDiscoverFunc() providerutil.DiscoverFunc {
	return func(ctx context.Context) ([]cluster.NodeMeta, error) {
		instances, err := p.cfg.Client.ListInstances(ctx, p.cfg.Project, p.cfg.Zone, p.cfg.LabelFilters)
		if err != nil {
			return nil, err
		}

		var members []cluster.NodeMeta
		for _, inst := range instances {
			if inst.Status != "RUNNING" {
				continue
			}
			if inst.PrivateIP == "" {
				continue
			}

			meta := cluster.NodeMeta{
				ID:       p.nodeID(inst),
				Addr:     p.addr(inst),
				Tags:     p.buildTags(inst),
				JoinedAt: time.Now(),
			}
			members = append(members, meta)
		}
		return members, nil
	}
}

func (p *GCPProvider) nodeID(info InstanceInfo) cluster.NodeID {
	if p.cfg.NodeIDFrom != nil {
		return p.cfg.NodeIDFrom(info)
	}
	return cluster.NodeID(info.Name)
}

func (p *GCPProvider) addr(info InstanceInfo) string {
	if p.cfg.AddrFunc != nil {
		return p.cfg.AddrFunc(info.PrivateIP, info)
	}
	return fmt.Sprintf("%s:%d", info.PrivateIP, p.cfg.Port)
}

func (p *GCPProvider) buildTags(info InstanceInfo) map[string]string {
	tags := map[string]string{
		"cloud.provider":      "gcp",
		"cloud.zone":          info.Zone,
		"cloud.instance-id":   info.ID,
		"cloud.instance-type": info.MachineType,
	}
	for k, v := range info.Labels {
		tags[k] = v
	}
	return tags
}
