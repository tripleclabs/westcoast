package integration_test

import (
	"context"
	"testing"

	"westcoast/src/actor"
)

func TestLookupByNameReturnsPID(t *testing.T) {
	rt := actor.NewRuntime()
	ref, _ := rt.CreateActor("lk1", 0, func(_ context.Context, state any, _ actor.Message) (any, error) { return state, nil })
	_, _ = ref.RegisterName("lookup-service")
	ack := rt.LookupName("lookup-service")
	assertLookupHit(t, ack, "lk1")
}
