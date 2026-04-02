# gcpprovider

GCE instance discovery for cluster membership.

## Install

```
go get github.com/tripleclabs/westcoast/pkg/gcpprovider
```

## Quick Start

```go
// Minimal — project and zone required
provider := gcpprovider.New("my-project", "us-central1-a")

// With label filters
provider := gcpprovider.NewWithConfig(gcpprovider.Config{
    Project: "my-project",
    Zone:    "us-central1-a",
    LabelFilters: map[string]string{
        "env":  "prod",
        "role": "worker",
    },
    PollInterval: 15 * time.Second,
})

provider.Start(selfMeta)
defer provider.Stop()
```

## Config Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `Project` | `string` | *required* | GCP project ID |
| `Zone` | `string` | *required* | GCE zone (e.g. `us-central1-a`) |
| `LabelFilters` | `map[string]string` | `nil` | GCE labels for filtering instances |
| `Port` | `int` | `9000` | Transport port in `NodeMeta.Addr` |
| `PollInterval` | `time.Duration` | `10s` | Time between GCE API polls |
| `FailureThreshold` | `int` | `3` | Missed polls before node is declared failed |
| `AddrFunc` | `func(ip, InstanceInfo) string` | `"ip:port"` | Custom address construction |
| `NodeIDFrom` | `func(InstanceInfo) NodeID` | instance name | Custom node ID derivation |
| `Client` | `InstanceLister` | real GCE client | Custom API implementation |
| `Probe` | `ProbeFunc` | `nil` | Health-check before including a node |

## Credentials

Uses GCP Application Default Credentials (ADC). On GCE instances this is automatic; locally, use `gcloud auth application-default login`.

## Tag Mapping

GCE instance labels are copied into `NodeMeta.Tags`. The provider also sets `cloud.provider`, `cloud.zone`, `cloud.instance-id`, and `cloud.instance-type` automatically.
