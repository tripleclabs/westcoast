package cluster

// PlacementAction describes what should happen to an actor from a failed node.
type PlacementAction int

const (
	// PlacementRestart creates the actor on the target node.
	PlacementRestart PlacementAction = iota
	// PlacementAbandon does nothing — the actor is not restarted.
	PlacementAbandon
)

// PlacementDecision describes where and how to restart an actor.
type PlacementDecision struct {
	ActorName  string
	TargetNode NodeID
	Action     PlacementAction
}

// ClusterSupervisionPolicy decides what to do when a node fails.
// Implementations can choose restart targets based on load, locality,
// or other criteria.
type ClusterSupervisionPolicy interface {
	// OnNodeFailed returns placement decisions for actors that were
	// on the failed node. The actorNames are the registered names
	// (from the registry) that belonged to the failed node.
	OnNodeFailed(failedNode NodeID, actorNames []string, liveMembers []NodeMeta) []PlacementDecision
}

// SimpleRestartPolicy restarts all actors on a round-robin selection
// of the remaining live nodes.
type SimpleRestartPolicy struct{}

func (SimpleRestartPolicy) OnNodeFailed(failedNode NodeID, actorNames []string, liveMembers []NodeMeta) []PlacementDecision {
	if len(liveMembers) == 0 || len(actorNames) == 0 {
		return nil
	}
	decisions := make([]PlacementDecision, len(actorNames))
	for i, name := range actorNames {
		decisions[i] = PlacementDecision{
			ActorName:  name,
			TargetNode: liveMembers[i%len(liveMembers)].ID,
			Action:     PlacementRestart,
		}
	}
	return decisions
}
