package unit_test

import (
	"testing"

	"westcoast/src/actor"
)

func TestPIDCanonicalShape(t *testing.T) {
	pid := actor.PID{Namespace: "default", ActorID: "a1", Generation: 1}
	if err := pid.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if pid.Key() != "default:a1" {
		t.Fatalf("key=%s", pid.Key())
	}
}

func TestPIDInvalidShape(t *testing.T) {
	cases := []actor.PID{{Namespace: "", ActorID: "x", Generation: 1}, {Namespace: "n", ActorID: "", Generation: 1}, {Namespace: "n", ActorID: "x", Generation: 0}}
	for _, c := range cases {
		if err := c.Validate(); err == nil {
			t.Fatalf("expected validation error for %+v", c)
		}
	}
}
