package actor

// SupervisorPolicy decides what action to take when an actor's handler fails.
type SupervisorPolicy interface {
	Decide(actorID string, err error, restartCount int) SupervisionDecision
}

// DefaultSupervisor is a SupervisorPolicy that restarts up to MaxRestarts times,
// then applies OnLimit (defaults to DecisionStop).
type DefaultSupervisor struct {
	MaxRestarts int
	OnLimit     SupervisionDecision
}

// Decide returns DecisionRestart if restartCount is below MaxRestarts,
// otherwise returns OnLimit or DecisionStop.
func (d DefaultSupervisor) Decide(_ string, _ error, restartCount int) SupervisionDecision {
	max := d.MaxRestarts
	if max < 0 {
		max = 1
	}
	if restartCount < max {
		return DecisionRestart
	}
	if d.OnLimit == DecisionEscalate {
		return DecisionEscalate
	}
	return DecisionStop
}
