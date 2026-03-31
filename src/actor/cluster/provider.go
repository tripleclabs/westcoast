package cluster

import "errors"

var (
	ErrProviderAlreadyStarted = errors.New("provider_already_started")
	ErrProviderNotStarted     = errors.New("provider_not_started")
)

// MemberEventType classifies cluster membership changes.
type MemberEventType int

const (
	MemberJoin    MemberEventType = iota // A new node has joined the cluster.
	MemberLeave                          // A node has gracefully left the cluster.
	MemberFailed                         // A node has been detected as failed (unresponsive).
	MemberUpdated                        // A node's metadata has changed.
)

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
