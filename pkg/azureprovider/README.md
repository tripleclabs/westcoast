# azureprovider

Azure VM discovery for cluster membership.

## Install

```
go get github.com/tripleclabs/westcoast/pkg/azureprovider
```

## Quick Start

```go
// Minimal — subscription and resource group required
provider := azureprovider.New("sub-id-here", "my-resource-group")

// With tag filters
provider := azureprovider.NewWithConfig(azureprovider.Config{
    SubscriptionID: "sub-id-here",
    ResourceGroup:  "my-resource-group",
    TagFilters: map[string]string{
        "cluster": "prod",
    },
    PollInterval: 15 * time.Second,
})

provider.Start(selfMeta)
defer provider.Stop()
```

## Config Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `SubscriptionID` | `string` | *required* | Azure subscription ID |
| `ResourceGroup` | `string` | *required* | Azure resource group |
| `TagFilters` | `map[string]string` | `nil` | Azure VM tags for filtering |
| `Port` | `int` | `9000` | Transport port in `NodeMeta.Addr` |
| `PollInterval` | `time.Duration` | `10s` | Time between Azure API polls |
| `FailureThreshold` | `int` | `3` | Missed polls before node is declared failed |
| `AddrFunc` | `func(ip, VMInfo) string` | `"ip:port"` | Custom address construction |
| `NodeIDFrom` | `func(VMInfo) NodeID` | VM name | Custom node ID derivation |
| `Client` | `VMLister` | real Azure client | Custom API implementation |
| `Probe` | `ProbeFunc` | `nil` | Health-check before including a node |

## Credentials

Uses `DefaultAzureCredential` from the Azure SDK (env vars, managed identity, Azure CLI, etc.).

## Tag Mapping

Azure VM tags are copied into `NodeMeta.Tags`. The provider also sets `cloud.provider`, `cloud.region`, `cloud.instance-id`, and `cloud.instance-type` automatically.

## Important: Private IP Resolution

The default `VMLister` returns the NIC resource ID as `PrivateIP`, not the actual private IP address. The Azure VM list API does not embed IP addresses directly -- resolving them requires a separate NIC GET call per interface.

For production use, provide a custom `VMLister` implementation (via `Config.Client`) that performs NIC lookups, or use an alternative mechanism such as VMSS instance view, IMDS, or Azure Resource Graph.
