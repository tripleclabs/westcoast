package cluster

import (
	"github.com/tripleclabs/westcoast/src/actor"
)

// RegistryStrategy defines the interface for distributed name registration.
// Implementations provide different consistency/availability tradeoffs.
type RegistryStrategy interface {
	// Register binds a name to a PID. Returns an error if the name is
	// already taken (exact semantics depend on the strategy).
	Register(name string, pid actor.PID) error

	// Lookup returns the PID bound to a name, or false if not found.
	Lookup(name string) (actor.PID, bool)

	// Unregister removes a name binding. Returns the PID that was
	// unregistered, or false if the name was not found.
	Unregister(name string) (actor.PID, bool)

	// UnregisterByNode removes all name bindings owned by actors on the
	// given node. Called when a node leaves or fails. Returns the names
	// that were unregistered.
	UnregisterByNode(node NodeID) []string

	// OnMembershipChange is called when the cluster membership changes.
	// Strategies that partition data or replicate state use this to
	// rebalance.
	OnMembershipChange(event MemberEvent)
}
