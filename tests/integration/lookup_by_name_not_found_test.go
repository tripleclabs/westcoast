package integration_test

import (
	"testing"

	"westcoast/src/actor"
)

func TestLookupByNameNotFound(t *testing.T) {
	rt := actor.NewRuntime()
	assertLookupMiss(t, rt.LookupName("missing-name"))
}
