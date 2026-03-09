package integration_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"westcoast/src/actor"
)

type askPing struct{}

func TestAskAsyncDelegationKeepsActorResponsive(t *testing.T) {
	rt := actor.NewRuntime()
	processed := make(chan struct{}, 1)
	var pingCount atomic.Int32
	ref, err := rt.CreateActor("ask-responsive", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		switch msg.Payload.(type) {
		case askPing:
			pingCount.Add(1)
			processed <- struct{}{}
			return state, nil
		}
		replyTo, ok := msg.AskReplyTo()
		if ok {
			requestID := msg.AskRequestID()
			go func() {
				time.Sleep(80 * time.Millisecond)
				_ = rt.SendPID(ctx, replyTo, askReply{Value: 5, RequestID: requestID})
			}()
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ref.Ask(context.Background(), askRequest{Value: 1}, 500*time.Millisecond)
	}()

	ack := ref.Send(context.Background(), askPing{})
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("send result=%s", ack.Result)
	}
	select {
	case <-processed:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("actor did not process non-ask message while async reply pending")
	}
	if pingCount.Load() == 0 {
		t.Fatal("ping was not processed")
	}
	<-done
}
