package cluster

import "context"

// PubSubAdapter handles cross-node publication broadcast.
// Following the Phoenix.PubSub model: subscriptions stay local on each
// node, only publications travel the network. Each node's local broker
// does trie matching and delivery to its own subscribers.
type PubSubAdapter interface {
	// Broadcast sends a publication to all other nodes in the cluster.
	// The payload is the raw Go value — the adapter encodes it.
	Broadcast(ctx context.Context, topic string, payload any, publisherNode NodeID) error

	// SetHandler registers the callback for incoming remote publications.
	// The callback should dispatch to the local broker without re-broadcasting.
	SetHandler(handler RemotePublishHandler)

	Start(ctx context.Context) error
	Stop() error
}

// RemotePublishHandler is called when a publication arrives from a remote node.
// The payload has been decoded back to a Go value.
type RemotePublishHandler func(topic string, payload any, publisherNode NodeID)
