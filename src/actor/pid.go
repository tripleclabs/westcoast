package actor

import (
	"fmt"
	"strings"
)

type PID struct {
	Namespace  string
	ActorID    string
	Generation uint64
}

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

type PIDDeliveryOutcome string

const (
	PIDDelivered               PIDDeliveryOutcome = "delivered"
	PIDUnresolved              PIDDeliveryOutcome = "unresolved"
	PIDRejectedStopped         PIDDeliveryOutcome = "rejected_stopped"
	PIDRejectedStaleGeneration PIDDeliveryOutcome = "rejected_stale_generation"
	PIDRejectedNotFound        PIDDeliveryOutcome = "rejected_not_found"
)

type PIDRouteState string

const (
	PIDRouteReachable  PIDRouteState = "reachable"
	PIDRouteUnresolved PIDRouteState = "unresolved"
	PIDRouteStopped    PIDRouteState = "stopped"
	PIDRouteRestarting PIDRouteState = "restarting"
)

type PIDSendAck struct {
	Outcome   PIDDeliveryOutcome
	MessageID uint64
}
