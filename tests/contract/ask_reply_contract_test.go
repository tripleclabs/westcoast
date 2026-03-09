package contract_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskReplyContract(t *testing.T) {
	outcomes := []actor.AskOutcomeType{
		actor.AskOutcomeSuccess,
		actor.AskOutcomeTimeout,
		actor.AskOutcomeCanceled,
		actor.AskOutcomeReplyTargetInvalid,
		actor.AskOutcomeLateReplyDropped,
	}
	if len(outcomes) != 5 {
		t.Fatalf("expected 5 ask outcomes, got %d", len(outcomes))
	}

	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-contract", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if !msg.IsAsk() {
			return state, nil
		}
		if msg.AskRequestID() == "" {
			t.Fatal("missing request_id")
		}
		replyTo, ok := msg.AskReplyTo()
		if !ok {
			t.Fatal("missing reply_to")
		}
		_ = rt.SendPID(ctx, replyTo, actor.AskReplyEnvelope{RequestID: msg.AskRequestID(), Payload: "ok"})
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	res, askErr := ref.Ask(context.Background(), "ping", time.Second)
	if askErr != nil {
		t.Fatalf("ask err: %v", askErr)
	}
	if res.RequestID == "" {
		t.Fatal("expected non-empty request id")
	}
	if res.Payload.(string) != "ok" {
		t.Fatalf("payload=%v", res.Payload)
	}

	slow, err := rt.CreateActor("ask-contract-slow", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		replyTo, _ := msg.AskReplyTo()
		requestID := msg.AskRequestID()
		go func() {
			time.Sleep(80 * time.Millisecond)
			_ = rt.SendPID(ctx, replyTo, actor.AskReplyEnvelope{RequestID: requestID, Payload: "late"})
		}()
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, askErr = slow.Ask(context.Background(), "slow", 20*time.Millisecond)
	if !errors.Is(askErr, actor.ErrAskTimeout) {
		t.Fatalf("expected timeout, got %v", askErr)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		outs := slow.AskOutcomes()
		hasLate := false
		for _, o := range outs {
			if o.Outcome == actor.AskOutcomeLateReplyDropped {
				hasLate = true
				break
			}
		}
		if hasLate {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected ask_late_reply_dropped outcome")
}
