package integration_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestGuardrailOutcomesEvidenceComplete(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	records := rt.ValidateDistributedReadiness()
	assertReadinessScope(t, records, actor.ReadinessScopePIDPolicy, actor.ReadinessPass)
	assertReadinessScope(t, records, actor.ReadinessScopeGatewayBoundary, actor.ReadinessPass)
	assertReadinessScope(t, records, actor.ReadinessScopeLocationTransparency, actor.ReadinessPass)
}
