package cluster

import (
	"context"
	"errors"
)

var (
	// ErrProviderAlreadyStarted is returned when Start is called on a running provider.
	ErrProviderAlreadyStarted = errors.New("provider_already_started")
	// ErrProviderNotStarted is returned when Stop is called on a provider that was never started.
	ErrProviderNotStarted = errors.New("provider_not_started")
)

// MemberEventType classifies cluster membership changes.
type MemberEventType int

const (
	MemberJoin    MemberEventType = iota // A new node has joined the cluster.
	MemberLeave                          // A node has gracefully left the cluster.
	MemberFailed                         // A node has been detected as failed (unresponsive).
	MemberUpdated                        // A node's metadata has changed.
)

// String returns a human-readable name for the event type.
func (t MemberEventType) String() string {
	switch t {
	case MemberJoin:
		return "join"
	case MemberLeave:
		return "leave"
	case MemberFailed:
		return "failed"
	case MemberUpdated:
		return "updated"
	default:
		return "unknown"
	}
}

// MemberEvent represents a change in cluster membership.
type MemberEvent struct {
	Type   MemberEventType
	Member NodeMeta
}

// BootstrappingProvider is a ClusterProvider that needs to run a bootstrap
// step before the cluster transport starts listening. Bootstrap is called
// by cluster.Start() before applying defaults for Transport and Auth —
// the provider returns pre-configured instances (e.g. with mTLS from a
// join handshake).
type BootstrappingProvider interface {
	ClusterProvider
	Bootstrap(ctx context.Context, self NodeMeta) (Transport, ClusterAuth, error)
}

// ClusterProvider handles node discovery and membership tracking.
// Implementations range from static seed lists to dynamic service discovery.
type ClusterProvider interface {
	// Start begins discovery using the provided local node metadata.
	// The provider emits MemberEvents as peers are discovered or lost.
	Start(self NodeMeta) error

	// Stop shuts down the provider and releases resources.
	Stop() error

	// Members returns the current known membership list (excluding self).
	Members() []NodeMeta

	// Events returns a channel that emits membership changes.
	// The channel is created by Start and closed by Stop.
	Events() <-chan MemberEvent
}
