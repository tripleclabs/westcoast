package awsprovider

import (
	"context"
	"fmt"
	"time"

	"github.com/tripleclabs/westcoast/pkg/providerutil"
	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// Compile-time check that AWSProvider satisfies ClusterProvider.
var _ cluster.ClusterProvider = (*AWSProvider)(nil)

// Config controls the AWS EC2 discovery provider.
type Config struct {
	// Region overrides the SDK default region (env / instance metadata).
	Region string

	// TagFilters are EC2 tag key=value pairs used to filter DescribeInstances.
	TagFilters map[string]string

	// Port is the transport port advertised in NodeMeta.Addr. Default: 9000.
	Port int

	// AddrFunc builds the address string from the instance's private IP and
	// info. When nil the default is "privateIP:port".
	AddrFunc func(ip string, info InstanceInfo) string

	// NodeIDFrom derives the cluster.NodeID from an instance. Default:
	// instance ID.
	NodeIDFrom func(info InstanceInfo) cluster.NodeID

	// PollInterval between discovery calls. Default: 10s.
	PollInterval time.Duration

	// FailureThreshold is how many consecutive missed polls before a node
	// is declared failed. Default: 3.
	FailureThreshold int

	// Client is the EC2 API abstraction. When nil a real SDK client is
	// created on Start.
	Client InstanceLister

	// Probe, if set, is used to health-check each discovered member before
	// it is included in the result set.
	Probe providerutil.ProbeFunc
}

func (c *Config) setDefaults() {
	if c.Port == 0 {
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
		c.AddrFunc = func(ip string, _ InstanceInfo) string {
			return fmt.Sprintf("%s:%d", ip, port)
		}
	}
	if c.NodeIDFrom == nil {
		c.NodeIDFrom = func(info InstanceInfo) cluster.NodeID {
			return cluster.NodeID(info.ID)
		}
	}
}

// AWSProvider discovers cluster members by polling EC2 DescribeInstances.
type AWSProvider struct {
	cfg    Config
	poller *providerutil.Poller
}

// New creates an AWSProvider with default configuration.
func New() *AWSProvider {
	return NewWithConfig(Config{})
}

// NewWithConfig creates an AWSProvider with the given configuration.
func NewWithConfig(cfg Config) *AWSProvider {
	cfg.setDefaults()
	return &AWSProvider{cfg: cfg}
}

// Start begins EC2 polling. If no Client was provided in Config, a real AWS
// SDK client is created using the default credential chain.
func (p *AWSProvider) Start(self cluster.NodeMeta) error {
	client := p.cfg.Client
	if client == nil {
		c, err := newDefaultClient(context.Background(), p.cfg.Region)
		if err != nil {
			return fmt.Errorf("awsprovider: %w", err)
		}
		client = c
	}

	discover := p.buildDiscover(client)
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
func (p *AWSProvider) Stop() error {
	if p.poller == nil {
		return cluster.ErrProviderNotStarted
	}
	return p.poller.Stop()
}

// Members returns the current set of alive peers.
func (p *AWSProvider) Members() []cluster.NodeMeta {
	if p.poller == nil {
		return nil
	}
	return p.poller.Members()
}

// Events returns the membership event channel.
func (p *AWSProvider) Events() <-chan cluster.MemberEvent {
	if p.poller == nil {
		return nil
	}
	return p.poller.Events()
}

// buildDiscover returns a DiscoverFunc that queries EC2 and maps instances
// to NodeMeta values.
func (p *AWSProvider) buildDiscover(client InstanceLister) providerutil.DiscoverFunc {
	cfg := p.cfg
	return func(ctx context.Context) ([]cluster.NodeMeta, error) {
		instances, err := client.ListInstances(ctx, cfg.TagFilters)
		if err != nil {
			return nil, err
		}

		var members []cluster.NodeMeta
		for _, inst := range instances {
			if inst.State != "running" {
				continue
			}

			tags := make(map[string]string, len(inst.Tags)+5)
			for k, v := range inst.Tags {
				tags[k] = v
			}
			tags["cloud.provider"] = "aws"
			tags["cloud.region"] = inst.Region
			tags["cloud.zone"] = inst.Zone
			tags["cloud.instance-id"] = inst.ID
			tags["cloud.instance-type"] = inst.InstanceType

			members = append(members, cluster.NodeMeta{
				ID:       cfg.NodeIDFrom(inst),
				Addr:     cfg.AddrFunc(inst.PrivateIP, inst),
				Tags:     tags,
				JoinedAt: time.Now(),
			})
		}
		return members, nil
	}
}
