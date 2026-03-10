package integration_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestNamedRegistrationDuplicateDeterminism(t *testing.T) {
	rt := actor.NewRuntime()
	refA, _ := rt.CreateActor("d1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	refB, _ := rt.CreateActor("d2", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	ack, err := refA.RegisterName("dup-service")
	assertRegisterSuccess(t, ack, err)
	if _, err := refB.RegisterName("dup-service"); err == nil {
		t.Fatal("expected duplicate registration error")
	}
}
