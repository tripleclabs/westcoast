package actor

type SupervisorPolicy interface {
	Decide(actorID string, err error, restartCount int) SupervisionDecision
}

type DefaultSupervisor struct {
	MaxRestarts int
	OnLimit     SupervisionDecision
}

func (d DefaultSupervisor) Decide(_ string, _ error, restartCount int) SupervisionDecision {
	max := d.MaxRestarts
	if max <= 0 {
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
