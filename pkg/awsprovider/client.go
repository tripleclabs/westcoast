// Package awsprovider implements a ClusterProvider that discovers nodes
// by polling AWS EC2 DescribeInstances.
package awsprovider

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// InstanceInfo holds the subset of EC2 instance data relevant to discovery.
type InstanceInfo struct {
	ID           string
	PrivateIP    string
	State        string // "running", "pending", "stopped", etc.
	Tags         map[string]string
	Region       string
	Zone         string
	InstanceType string
}

// InstanceLister abstracts the EC2 API so callers can supply a mock.
type InstanceLister interface {
	ListInstances(ctx context.Context, filters map[string]string) ([]InstanceInfo, error)
}

// defaultClient wraps the real AWS EC2 SDK v2 client.
type defaultClient struct {
	ec2    *ec2.Client
	region string
}

// newDefaultClient creates a defaultClient using config.LoadDefaultConfig.
// If region is non-empty it overrides the SDK default.
func newDefaultClient(ctx context.Context, region string) (*defaultClient, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	return &defaultClient{
		ec2:    ec2.NewFromConfig(cfg),
		region: cfg.Region,
	}, nil
}

// ListInstances calls ec2.DescribeInstances with the given tag filters and
// returns a flat list of InstanceInfo.
func (c *defaultClient) ListInstances(ctx context.Context, filters map[string]string) ([]InstanceInfo, error) {
	input := &ec2.DescribeInstancesInput{}

	var sdkFilters []types.Filter
	for k, v := range filters {
		name := "tag:" + k
		value := v
		sdkFilters = append(sdkFilters, types.Filter{
			Name:   &name,
			Values: []string{value},
		})
	}
	if len(sdkFilters) > 0 {
		input.Filters = sdkFilters
	}

	var instances []InstanceInfo

	paginator := ec2.NewDescribeInstancesPaginator(c.ec2, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("ec2 describe instances: %w", err)
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				info := InstanceInfo{
					Region: c.region,
				}
				if inst.InstanceId != nil {
					info.ID = *inst.InstanceId
				}
				if inst.PrivateIpAddress != nil {
					info.PrivateIP = *inst.PrivateIpAddress
				}
				if inst.State != nil {
					info.State = string(inst.State.Name)
				}
				if inst.Placement != nil && inst.Placement.AvailabilityZone != nil {
					info.Zone = *inst.Placement.AvailabilityZone
				}
				info.InstanceType = string(inst.InstanceType)

				tags := make(map[string]string, len(inst.Tags))
				for _, t := range inst.Tags {
					if t.Key != nil && t.Value != nil {
						tags[*t.Key] = *t.Value
					}
				}
				info.Tags = tags

				instances = append(instances, info)
			}
		}
	}
	return instances, nil
}
