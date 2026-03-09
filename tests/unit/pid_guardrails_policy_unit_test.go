package unit_test

import (
	"testing"

	"westcoast/src/actor"
)

func TestPIDPolicyDefaultDisabled(t *testing.T) {
	rt := actor.NewRuntime()
	if rt.PIDInteractionPolicy() != actor.PIDInteractionPolicyDisabled {
		t.Fatalf("policy=%s", rt.PIDInteractionPolicy())
	}
}

func TestPIDPolicyCanEnable(t *testing.T) {
	rt := actor.NewRuntime()
	rt.SetPIDInteractionPolicy(actor.PIDInteractionPolicyPIDOnly)
	if rt.PIDInteractionPolicy() != actor.PIDInteractionPolicyPIDOnly {
		t.Fatalf("policy=%s", rt.PIDInteractionPolicy())
	}
}
