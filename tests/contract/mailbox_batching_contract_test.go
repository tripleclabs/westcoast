package contract_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

type contractBatchReceiver struct{}

func (contractBatchReceiver) BatchReceive(_ context.Context, state any, payloads []any) (any, error) {
	count := 0
	if state != nil {
		count = state.(int)
	}
	return count + len(payloads), nil
}

type contractFailBatchReceiver struct{}

func (contractFailBatchReceiver) BatchReceive(_ context.Context, state any, _ []any) (any, error) {
	return state, errors.New("batch_fail")
}

func TestMailboxBatchingContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("batch-contract", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(8, contractBatchReceiver{}))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 24; i++ {
		ack := ref.Send(context.Background(), i)
		if ack.Result != actor.SubmitAccepted {
			t.Fatalf("send rejected: %s", ack.Result)
		}
	}
	waitForContract(t, time.Second, func() bool {
		outs := ref.BatchOutcomes()
		if len(outs) == 0 {
			return false
		}
		total := 0
		for _, out := range outs {
			total += out.BatchSize
		}
		return total >= 24
	})
	outs := ref.BatchOutcomes()
	for _, out := range outs {
		if out.BatchSize <= 0 || out.BatchSize > 8 {
			t.Fatalf("invalid batch size: %+v", out)
		}
	}
}

func TestMailboxBatchingFailureContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("batch-contract-fail", 0, func(_ context.Context, state any, _ actor.Message) (any, error) {
		return state, nil
	}, actor.WithBatching(4, contractFailBatchReceiver{}))
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.Send(context.Background(), 1)
	if ack.Result != actor.SubmitAccepted {
		t.Fatalf("send rejected: %s", ack.Result)
	}
	waitForContract(t, time.Second, func() bool {
		outs := ref.BatchOutcomes()
		for _, out := range outs {
			if out.Result == actor.BatchResultFailedHandler {
				return true
			}
		}
		return false
	})
}

func waitForContract(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met")
}
