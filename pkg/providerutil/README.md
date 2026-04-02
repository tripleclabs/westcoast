# providerutil

Shared polling infrastructure for building `ClusterProvider` implementations.

## Install

```
go get github.com/tripleclabs/westcoast/pkg/providerutil
```

## Quick Start

```go
discover := func(ctx context.Context) ([]cluster.NodeMeta, error) {
    // Call your API, return discovered nodes.
    return nodes, nil
}

poller := providerutil.NewPoller(discover, providerutil.PollerConfig{
    Interval:         5 * time.Second,
    FailureThreshold: 3,
})

poller.Start(selfMeta)
defer poller.Stop()

// Use poller.Members() and poller.Events() as usual.
```

### WithProbe

Wrap a `DiscoverFunc` to health-check each node before reporting it alive:

```go
probe := func(ctx context.Context, addr string) error {
    conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
    if err != nil {
        return err
    }
    return conn.Close()
}

discover = providerutil.WithProbe(discover, probe)
poller := providerutil.NewPoller(discover, providerutil.PollerConfig{})
```

## Config Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `Interval` | `time.Duration` | `10s` | Time between discovery polls |
| `FailureThreshold` | `int` | `3` | Consecutive missed polls before a node is declared failed |
| `EventBufferSize` | `int` | `1024` | Capacity of the `MemberEvent` channel |
