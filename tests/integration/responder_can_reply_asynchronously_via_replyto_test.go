package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestResponderCanReplyAsynchronouslyViaReplyTo(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-async", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		replyTo, ok := msg.AskReplyTo()
		if !ok {
			return state, nil
		}
		requestID := msg.AskRequestID()
		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = rt.SendPID(ctx, replyTo, askReply{Value: 99, RequestID: requestID})
		}()
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	res, askErr := ref.Ask(context.Background(), askRequest{Value: 1}, time.Second)
	if askErr != nil {
		t.Fatal(askErr)
	}
	if res.Payload.(askReply).Value != 99 {
		t.Fatalf("unexpected payload: %+v", res.Payload)
	}
}
