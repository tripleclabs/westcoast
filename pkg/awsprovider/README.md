# awsprovider

EC2 instance discovery via tags for cluster membership.

## Install

```
go get github.com/tripleclabs/westcoast/pkg/awsprovider
```

## Quick Start

```go
// Minimal — uses IAM role credentials, discovers all running EC2 instances
provider := awsprovider.New()

// With tag filters
provider := awsprovider.NewWithConfig(awsprovider.Config{
    Region: "us-west-2",
    TagFilters: map[string]string{
        "cluster": "prod",
        "role":    "worker",
    },
    Port:         8080,
    PollInterval: 15 * time.Second,
})

provider.Start(selfMeta)
defer provider.Stop()
```

## Config Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `Region` | `string` | SDK default | Override the AWS region |
| `TagFilters` | `map[string]string` | `nil` | EC2 tag key=value pairs for filtering |
| `Port` | `int` | `9000` | Transport port in `NodeMeta.Addr` |
| `PollInterval` | `time.Duration` | `10s` | Time between EC2 API polls |
| `FailureThreshold` | `int` | `3` | Missed polls before node is declared failed |
| `AddrFunc` | `func(ip, InstanceInfo) string` | `"ip:port"` | Custom address construction |
| `NodeIDFrom` | `func(InstanceInfo) NodeID` | instance ID | Custom node ID derivation |
| `Client` | `InstanceLister` | real SDK client | Custom EC2 API implementation |
| `Probe` | `ProbeFunc` | `nil` | Health-check before including a node |

## Credentials

Uses the AWS SDK v2 default credential chain (env vars, shared config, IAM role, IMDS).

## Tag Mapping

EC2 instance tags are copied into `NodeMeta.Tags`. The provider also sets `cloud.provider`, `cloud.region`, `cloud.zone`, `cloud.instance-id`, and `cloud.instance-type` automatically.
