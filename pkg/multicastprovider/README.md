# multicastprovider

UDP multicast LAN discovery for cluster membership.

## Install

```
go get github.com/tripleclabs/westcoast/pkg/multicastprovider
```

## Quick Start

```go
// Default config (group 239.1.1.1:7946, port 9000)
provider := multicastprovider.New()

// Or with custom config
provider := multicastprovider.NewWithConfig(multicastprovider.Config{
    GroupAddr:         "239.1.1.1:7946",
    BroadcastInterval: 2 * time.Second,
    DeadTimeout:        10 * time.Second,
    Port:               8080,
})

provider.Start(selfMeta)
defer provider.Stop()

for ev := range provider.Events() {
    fmt.Println(ev.Type, ev.Member.ID)
}
```

## Config Reference

| Field | Type | Default | Description |
|---|---|---|---|
| `GroupAddr` | `string` | `"239.1.1.1:7946"` | Multicast group address and port |
| `Interface` | `string` | `""` (all) | Network interface name to bind |
| `BroadcastInterval` | `time.Duration` | `1s` | How often this node announces itself |
| `DeadTimeout` | `time.Duration` | `5s` | Silence duration before a node is declared failed |
| `Port` | `int` | `9000` | Transport port advertised in `NodeMeta.Addr` |

## Notes

- **Graceful leave**: `Stop()` sends a leave packet so peers detect departure immediately instead of waiting for `DeadTimeout`.
- **Failure detection**: If a node goes silent for longer than `DeadTimeout`, it is declared failed via a `MemberFailed` event.
- **Tag updates**: Call `UpdateTags()` to change announced tags at runtime; peers will receive `MemberUpdated` events.
