package unit_test

import "westcoast/src/actor"

func hasBrokerOutcome(outcomes []actor.BrokerOutcome, want actor.BrokerOutcomeType) bool {
	for _, out := range outcomes {
		if out.Result == want {
			return true
		}
	}
	return false
}
