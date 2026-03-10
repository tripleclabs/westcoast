package unit_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestDefaultSupervisorDecisionLifecycle(t *testing.T) {
	s := actor.DefaultSupervisor{MaxRestarts: 1}
	if d := s.Decide("x", nil, 0); d != actor.DecisionRestart {
		t.Fatalf("first decision=%s", d)
	}
	if d := s.Decide("x", nil, 1); d != actor.DecisionStop {
		t.Fatalf("second decision=%s", d)
	}
}

func TestDefaultSupervisorEscalateOnLimit(t *testing.T) {
	s := actor.DefaultSupervisor{MaxRestarts: 1, OnLimit: actor.DecisionEscalate}
	if d := s.Decide("x", nil, 1); d != actor.DecisionEscalate {
		t.Fatalf("decision=%s", d)
	}
}
