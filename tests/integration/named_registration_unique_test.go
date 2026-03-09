package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestNamedRegistrationUnique(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("u1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	if err != nil {
		t.Fatal(err)
	}
	ack, err := ref.RegisterName("user-service-123")
	assertRegisterSuccess(t, ack, err)
	assertLookupHit(t, rt.LookupName("user-service-123"), "u1")
}
