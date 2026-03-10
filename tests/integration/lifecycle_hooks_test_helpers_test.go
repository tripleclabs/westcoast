package integration_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func assertLifecycleResult(t *testing.T, outcomes []actor.LifecycleHookOutcome, phase actor.LifecycleHookPhase, result actor.LifecycleHookResult) {
	t.Helper()
	for _, o := range outcomes {
		if o.Phase == phase && o.Result == result {
			return
		}
	}
	t.Fatalf("missing lifecycle outcome phase=%s result=%s", phase, result)
}
