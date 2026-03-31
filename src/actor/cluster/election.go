package cluster

// LeaderElection provides scoped leader election across cluster members.
// Multiple independent elections can run concurrently (different scopes).
// Each election has a monotonically increasing term number that serves
// as a fencing token for distributed decisions.
type LeaderElection interface {
	// Leader returns the current leader for the scope, or ("", false)
	// if no leader is elected.
	Leader(scope string) (NodeID, bool)

	// IsLeader returns true if this node is the leader for the scope.
	IsLeader(scope string) bool

	// Term returns the current election term for the scope.
	// The term increments each time leadership changes.
	Term(scope string) uint64

	// Watch returns a channel that emits on leadership changes for the scope.
	// The channel is unbuffered or lightly buffered — consumers should
	// drain it promptly.
	Watch(scope string) <-chan LeaderEvent

	// OnMembershipChange updates the election state based on cluster changes.
	// This may trigger a new election if the current leader left/failed.
	OnMembershipChange(event MemberEvent)
}

// LeaderEvent is emitted when leadership changes for a scope.
type LeaderEvent struct {
	Scope      string
	Leader     NodeID
	PrevLeader NodeID
	Term       uint64
}
