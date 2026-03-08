package actor

import (
	"fmt"
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
