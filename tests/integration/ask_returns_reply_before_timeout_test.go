package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskReturnsReplyBeforeTimeout(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-echo", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if !msg.IsAsk() {
			return state, nil
		}
		replyTo, ok := msg.AskReplyTo()
		if !ok {
			t.Fatalf("missing reply_to")
		}
		req := msg.Payload.(askRequest)
		_ = rt.SendPID(ctx, replyTo, askReply{Value: req.Value + 1, RequestID: msg.AskRequestID()})
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	res, askErr := ref.Ask(context.Background(), askRequest{Value: 41}, 500*time.Millisecond)
	if askErr != nil {
		t.Fatalf("ask error: %v", askErr)
	}
	got := res.Payload.(askReply)
	if got.Value != 42 {
		t.Fatalf("reply value=%d", got.Value)
	}
	if got.RequestID == "" {
		t.Fatal("missing request_id in reply")
	}
	waitForAskOutcome(t, ref, time.Second, actor.AskOutcomeSuccess)
}
