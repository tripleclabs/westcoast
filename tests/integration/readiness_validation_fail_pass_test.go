package integration_test

import (
	"testing"

	"westcoast/src/actor"
)

func TestReadinessValidationFailThenPass(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyDisabled)
	records := rt.ValidateDistributedReadiness()
	assertReadinessScope(t, records, actor.ReadinessScopePIDPolicy, actor.ReadinessFail)

	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	records = rt.ValidateDistributedReadiness()
	assertReadinessScope(t, records, actor.ReadinessScopePIDPolicy, actor.ReadinessPass)
}
