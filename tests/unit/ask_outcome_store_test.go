package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskOutcomesRecordTerminalStates(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-outcomes-success", 0, func(ctx context.Context, state any, msg actor.Message) (any, error) {
		if !msg.IsAsk() {
			return state, nil
		}
		replyTo, _ := msg.AskReplyTo()
		_ = rt.SendPID(ctx, replyTo, 10)
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, askErr := ref.Ask(context.Background(), 1, 500*time.Millisecond); askErr != nil {
		t.Fatalf("ask err: %v", askErr)
	}
	timeoutRef, err := rt.CreateActor("ask-outcomes-timeout", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, askErr := timeoutRef.Ask(context.Background(), 1, 10*time.Millisecond); !errors.Is(askErr, actor.ErrAskTimeout) {
		t.Fatalf("expected timeout err, got %v", askErr)
	}

	outs := ref.AskOutcomes()
	if len(outs) < 1 {
		t.Fatalf("expected >=1 ask outcome, got %d", len(outs))
	}
}
