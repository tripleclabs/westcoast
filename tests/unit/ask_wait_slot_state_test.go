package unit_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestAskWaitSlotTimeoutAndCancelTransitions(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-state", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	_, timeoutErr := ref.Ask(context.Background(), 1, 20*time.Millisecond)
	if !errors.Is(timeoutErr, actor.ErrAskTimeout) {
		t.Fatalf("expected timeout error, got %v", timeoutErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, cancelErr := ref.Ask(ctx, 1, time.Second)
	if !errors.Is(cancelErr, actor.ErrAskCanceled) {
		t.Fatalf("expected canceled error, got %v", cancelErr)
	}
}

func TestAskRejectsInvalidTimeout(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ask-timeout-invalid", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	_, askErr := ref.Ask(context.Background(), 1, 0)
	if !errors.Is(askErr, actor.ErrAskInvalidTimeout) {
		t.Fatalf("expected invalid timeout, got %v", askErr)
	}
}
