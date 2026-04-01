package actor

import (
	"fmt"
	"strings"
)

// PID is a process identifier that uniquely addresses an actor within a namespace and generation.
type PID struct {
	Namespace  string
	ActorID    string
	Generation uint64
}

// Validate returns an error if the PID is missing required fields.
func (p PID) Validate() error {
	if p.Namespace == "" {
		return fmt.Errorf("namespace required")
	}
	if p.ActorID == "" {
		return fmt.Errorf("actor_id required")
	}
	if p.Generation == 0 {
		return fmt.Errorf("generation must be >=1")
	}
	return nil
}

// Key returns a string key in the form "namespace:actorID" for map lookups.
func (p PID) Key() string {
	return fmt.Sprintf("%s:%s", p.Namespace, p.ActorID)
}

// IsRemote returns true if this PID belongs to a different node.
// A PID is remote if its namespace is non-empty, differs from the local
// namespace, and is not an internal ask-reply namespace.
func (p PID) IsRemote(localNamespace string) bool {
	if p.Namespace == "" || p.Namespace == localNamespace {
		return false
	}
	// Plain ask-reply (no node qualifier) is always local.
	if p.Namespace == "__ask_reply" {
		return false
	}
	// Node-qualified ask-reply for the local node is local.
	if localNamespace != "" && p.Namespace == "__ask_reply@"+localNamespace {
		return false
	}
	// Everything else is remote (including ask-replies for other nodes).
	return true
}

// isAskReplyNamespace returns true if the namespace is an ask-reply
// namespace (either plain or node-qualified).
func isAskReplyNamespace(ns string) bool {
	return ns == "__ask_reply" || strings.HasPrefix(ns, "__ask_reply@")
}

// PIDDeliveryOutcome describes the result of sending a message to a PID.
type PIDDeliveryOutcome string

const (
	// PIDDelivered indicates the message was successfully delivered.
	PIDDelivered PIDDeliveryOutcome = "delivered"
	// PIDUnresolved indicates the PID could not be resolved.
	PIDUnresolved PIDDeliveryOutcome = "unresolved"
	// PIDRejectedStopped indicates the target actor is stopped.
	PIDRejectedStopped PIDDeliveryOutcome = "rejected_stopped"
	// PIDRejectedStaleGeneration indicates the PID's generation is outdated.
	PIDRejectedStaleGeneration PIDDeliveryOutcome = "rejected_stale_generation"
	// PIDRejectedNotFound indicates the target actor does not exist.
	PIDRejectedNotFound PIDDeliveryOutcome = "rejected_not_found"
)

// PIDRouteState describes the routing availability of a PID.
type PIDRouteState string

const (
	// PIDRouteReachable indicates the actor is available to receive messages.
	PIDRouteReachable PIDRouteState = "reachable"
	// PIDRouteUnresolved indicates the PID has not been registered.
	PIDRouteUnresolved PIDRouteState = "unresolved"
	// PIDRouteStopped indicates the actor has been stopped.
	PIDRouteStopped PIDRouteState = "stopped"
	// PIDRouteRestarting indicates the actor is being restarted.
	PIDRouteRestarting PIDRouteState = "restarting"
)

// PIDSendAck is the acknowledgment returned after sending a message to a PID.
type PIDSendAck struct {
	Outcome   PIDDeliveryOutcome
	MessageID uint64
}
