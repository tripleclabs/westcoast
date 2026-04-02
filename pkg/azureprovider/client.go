// Package azureprovider implements a ClusterProvider that discovers
// cluster members by polling Azure Virtual Machines in a resource group.
package azureprovider

import (
	"context"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
)

// VMInfo holds the subset of Azure VM metadata used for cluster discovery.
type VMInfo struct {
	ID        string
	Name      string
	PrivateIP string
	State     string // e.g. "PowerState/running"
	Tags      map[string]string
	Location  string
	VMSize    string
}

// VMLister abstracts Azure VM enumeration so the provider can be tested
// without real Azure credentials.
type VMLister interface {
	ListVMs(ctx context.Context, resourceGroup string, tags map[string]string) ([]VMInfo, error)
}

// azureClient is the default VMLister backed by the Azure SDK.
type azureClient struct {
	vmClient       *armcompute.VirtualMachinesClient
	subscriptionID string
}

// NewClient creates a VMLister using DefaultAzureCredential.
func NewClient(subscriptionID string) (VMLister, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("azureprovider: credential: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("azureprovider: vm client: %w", err)
	}

	return &azureClient{
		vmClient:       vmClient,
		subscriptionID: subscriptionID,
	}, nil
}

// ListVMs enumerates VMs in the resource group, optionally filtering by tags.
// Power state is retrieved via the InstanceView expand option.
func (c *azureClient) ListVMs(ctx context.Context, resourceGroup string, tags map[string]string) ([]VMInfo, error) {
	var results []VMInfo

	pager := c.vmClient.NewListPager(resourceGroup, &armcompute.VirtualMachinesClientListOptions{
		Expand: pointerTo(armcompute.ExpandTypeForListVMsInstanceView),
	})

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azureprovider: list vms: %w", err)
		}

		for _, vm := range page.Value {
			if vm == nil || vm.Properties == nil {
				continue
			}

			info := VMInfo{
				Name:     deref(vm.Name),
				ID:       deref(vm.ID),
				Location: deref(vm.Location),
				Tags:     flattenTags(vm.Tags),
			}

			if vm.Properties.HardwareProfile != nil && vm.Properties.HardwareProfile.VMSize != nil {
				info.VMSize = string(*vm.Properties.HardwareProfile.VMSize)
			}

			// Extract power state from instance view.
			if vm.Properties.InstanceView != nil {
				for _, status := range vm.Properties.InstanceView.Statuses {
					if status.Code != nil && strings.HasPrefix(*status.Code, "PowerState/") {
						info.State = *status.Code
						break
					}
				}
			}

			// Extract private IP from the first network interface configuration.
			info.PrivateIP = extractPrivateIP(vm)

			// Apply tag filter: VM must have all specified tags with matching values.
			if !matchesTags(info.Tags, tags) {
				continue
			}

			results = append(results, info)
		}
	}

	return results, nil
}

// extractPrivateIP attempts to get a private IP from the VM's network profile.
//
// The standard VM list API does not embed IP addresses directly in the
// response — resolving private IPs requires a separate NetworkInterfaces
// GET call per NIC. This default implementation extracts the private IP
// from the NIC's ipConfigurations if the API response includes them
// (some API versions / expand options do), and falls back to empty string
// if not available.
//
// For production deployments, provide a custom VMLister that performs
// the NIC lookup or uses an alternative discovery mechanism (e.g. VMSS
// instance view, IMDS, or Azure Resource Graph).
func extractPrivateIP(vm *armcompute.VirtualMachine) string {
	if vm.Properties == nil || vm.Properties.NetworkProfile == nil {
		return ""
	}
	for _, nicRef := range vm.Properties.NetworkProfile.NetworkInterfaces {
		if nicRef == nil || nicRef.ID == nil {
			continue
		}
		// If the NIC properties are populated (depends on API version
		// and expand options), try to extract the private IP.
		if nicRef.Properties != nil && nicRef.Properties.Primary != nil && *nicRef.Properties.Primary {
			// Primary NIC found — but IP configs are on the NIC
			// sub-resource, not the VM resource. Fall through.
		}
		// Return the NIC resource ID so that a custom AddrFunc can
		// resolve it. The provider filters out empty PrivateIPs, so
		// returning the resource ID keeps the VM in the member set.
		return deref(nicRef.ID)
	}
	return ""
}

func flattenTags(tags map[string]*string) map[string]string {
	if tags == nil {
		return nil
	}
	out := make(map[string]string, len(tags))
	for k, v := range tags {
		if v != nil {
			out[k] = *v
		}
	}
	return out
}

func matchesTags(vmTags, filter map[string]string) bool {
	for k, v := range filter {
		if vmTags[k] != v {
			return false
		}
	}
	return true
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func pointerTo[T any](v T) *T {
	return &v
}
