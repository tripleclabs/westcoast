package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestLateReplyIsDroppedAfterTimeout(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-late", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		replyTo, ok := msg.AskReplyTo()
		if ok {
			requestID := msg.AskRequestID()
			go func() {
				time.Sleep(100 * time.Millisecond)
				_ = rt.SendPID(ctx, replyTo, askReply{Value: 1, RequestID: requestID})
			}()
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, askErr := ref.Ask(context.Background(), askRequest{Value: 1}, 20*time.Millisecond)
	if !errors.Is(askErr, actor.ErrAskTimeout) {
		t.Fatalf("expected timeout, got %v", askErr)
	}
	waitFor(t, time.Second, func() bool {
		outs := ref.AskOutcomes()
		hasTimeout := false
		hasLateDrop := false
		for _, o := range outs {
			if o.Outcome == actor.AskOutcomeTimeout {
				hasTimeout = true
			}
			if o.Outcome == actor.AskOutcomeLateReplyDropped {
				hasLateDrop = true
			}
		}
		return hasTimeout && hasLateDrop
	}, "expected timeout and late-drop outcomes")
}
