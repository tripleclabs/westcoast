// Package gcpprovider implements a ClusterProvider that discovers cluster
// members by polling the GCE instances API. It uses Application Default
// Credentials and the shared providerutil.Poller for polling and diffing.
package gcpprovider

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

// InstanceInfo holds the relevant metadata for a GCE instance.
type InstanceInfo struct {
	ID          string
	Name        string
	PrivateIP   string
	Status      string // "RUNNING", "STAGING", "TERMINATED", etc.
	Labels      map[string]string
	Zone        string
	MachineType string
}

// InstanceLister abstracts the GCE instance listing API. This allows
// unit tests to substitute a mock without real GCP credentials.
type InstanceLister interface {
	ListInstances(ctx context.Context, project, zone string, labels map[string]string) ([]InstanceInfo, error)
}

// gceClient is the default InstanceLister backed by the GCE REST API.
type gceClient struct {
	client *compute.InstancesClient
}

// NewClient creates an InstanceLister that calls the real GCE instances API
// using Application Default Credentials.
func NewClient(ctx context.Context) (InstanceLister, error) {
	c, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("gcpprovider: create instances client: %w", err)
	}
	return &gceClient{client: c}, nil
}

// buildFilter constructs a GCE filter expression from label key-value pairs.
// Example output: "labels.env=prod AND labels.role=worker".
func buildFilter(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, fmt.Sprintf("labels.%s=%s", k, v))
	}
	return strings.Join(parts, " AND ")
}

// extractMachineType returns the short machine type name from a full
// resource URL like "zones/us-central1-a/machineTypes/e2-medium".
func extractMachineType(fullURL string) string {
	if i := strings.LastIndex(fullURL, "/"); i >= 0 {
		return fullURL[i+1:]
	}
	return fullURL
}

func (c *gceClient) ListInstances(ctx context.Context, project, zone string, labels map[string]string) ([]InstanceInfo, error) {
	req := &computepb.ListInstancesRequest{
		Project: project,
		Zone:    zone,
	}
	if f := buildFilter(labels); f != "" {
		req.Filter = &f
	}

	var instances []InstanceInfo
	it := c.client.List(ctx, req)
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcpprovider: list instances: %w", err)
		}

		info := InstanceInfo{
			Name:   inst.GetName(),
			Status: inst.GetStatus(),
			Labels: inst.GetLabels(),
			Zone:   zone,
		}
		if inst.Id != nil {
			info.ID = fmt.Sprintf("%d", inst.GetId())
		}
		if nics := inst.GetNetworkInterfaces(); len(nics) > 0 {
			info.PrivateIP = nics[0].GetNetworkIP()
		}
		if mt := inst.GetMachineType(); mt != "" {
			info.MachineType = extractMachineType(mt)
		}

		instances = append(instances, info)
	}
	return instances, nil
}
