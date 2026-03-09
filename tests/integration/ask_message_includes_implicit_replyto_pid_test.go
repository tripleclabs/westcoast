package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskMessageIncludesImplicitReplyToPID(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-context", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if !msg.IsAsk() {
			t.Fatal("expected ask message")
		}
		if msg.AskRequestID() == "" {
			t.Fatal("missing request_id")
		}
		replyTo, ok := msg.AskReplyTo()
		if !ok {
			t.Fatal("missing reply_to")
		}
		if replyTo.Namespace == "" {
			t.Fatal("missing reply_to namespace")
		}
		_ = rt.SendPID(ctx, replyTo, askReply{Value: 7, RequestID: msg.AskRequestID()})
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	res, askErr := ref.Ask(context.Background(), askRequest{Value: 7}, 500*time.Millisecond)
	if askErr != nil {
		t.Fatal(askErr)
	}
	if res.Payload.(askReply).Value != 7 {
		t.Fatalf("unexpected response: %+v", res.Payload)
	}
}
