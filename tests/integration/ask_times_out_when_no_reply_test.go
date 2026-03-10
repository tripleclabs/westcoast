package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestAskTimesOutWhenNoReply(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-timeout", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, askErr := ref.Ask(context.Background(), askRequest{Value: 1}, 30*time.Millisecond)
	if !errors.Is(askErr, actor.ErrAskTimeout) {
		t.Fatalf("expected ErrAskTimeout, got %v", askErr)
	}
	waitForAskOutcome(t, ref, time.Second, actor.AskOutcomeTimeout)
}
