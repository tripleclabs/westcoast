package integration_test

import (
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type askRequest struct {
	Value int
}

type askReply struct {
	Value     int
	RequestID string
}

func waitForAskOutcome(t *testing.T, ref *actor.ActorRef, timeout time.Duration, want actor.AskOutcomeType) {
	t.Helper()
	waitFor(t, timeout, func() bool {
		outs := ref.AskOutcomes()
		if len(outs) == 0 {
			return false
		}
		return outs[len(outs)-1].Outcome == want
	}, "expected ask outcome "+string(want))
}
