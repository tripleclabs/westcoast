package integration_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskConcurrentRequestsDoNotCrossWireReplies(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-concurrent", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		req := msg.Payload.(askRequest)
		replyTo, ok := msg.AskReplyTo()
		if !ok {
			t.Fatal("missing reply_to")
		}
		_ = rt.SendPID(ctx, replyTo, askReply{Value: req.Value, RequestID: msg.AskRequestID()})
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	const n = 100
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, askErr := ref.Ask(context.Background(), askRequest{Value: i}, 2*time.Second)
			if askErr != nil {
				errs <- askErr
				return
			}
			rep := res.Payload.(askReply)
			if rep.Value != i {
				errs <- context.DeadlineExceeded
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ask failed: %v", err)
		}
	}
}
