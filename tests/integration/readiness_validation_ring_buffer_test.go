package integration_test

import (
	"fmt"
	"testing"

	"westcoast/src/actor"
)

func TestReadinessValidationUsesBoundedHistory(t *testing.T) {
	rt := actor.NewRuntime()
	for i := 0; i < 120; i++ {
		rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyMode(fmt.Sprintf("mode-%d", i)))
		_ = rt.ValidateDistributedReadiness()
	}

	records := rt.ValidateDistributedReadiness()
	if len(records) != 256 {
		t.Fatalf("expected bounded readiness history length 256, got %d", len(records))
	}
	last := records[len(records)-1]
	if last.Scope != actor.ReadinessScopeLocationTransparency {
		t.Fatalf("expected newest readiness scope location_transparency, got %s", last.Scope)
	}
}
