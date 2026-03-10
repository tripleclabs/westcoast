package integration_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestLookupByNameNotFound(t *testing.T) {
	rt := actor.NewRuntime()
	assertLookupMiss(t, rt.LookupName("missing-name"))
}
