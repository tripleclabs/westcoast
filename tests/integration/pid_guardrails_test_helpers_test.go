package integration_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func assertGuardrailOutcome(t *testing.T, outs []actor.GuardrailOutcome, want actor.GuardrailOutcomeType) {
	t.Helper()
	for _, out := range outs {
		if out.Outcome == want {
			return
		}
	}
	t.Fatalf("missing guardrail outcome=%s", want)
}

func assertReadinessScope(t *testing.T, records []actor.ReadinessValidationRecord, scope actor.ReadinessScope, want actor.ReadinessResult) {
	t.Helper()
	for _, r := range records {
		if r.Scope == scope && r.Result == want {
			return
		}
	}
	t.Fatalf("missing readiness scope=%s result=%s", scope, want)
}
