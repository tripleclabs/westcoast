package integration_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

type pidInc struct{ N int }
type pidGet struct{ Ch chan int }

func pidCounter(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.(int)
	switch p := msg.Payload.(type) {
	case pidInc:
		return cur + p.N, nil
	case pidGet:
		p.Ch <- cur
		return cur, nil
	default:
		return cur, nil
	}
}

func TestPIDOnlyDelivery(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("pid-a", 0, pidCounter)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := ref.PID("default")
	if err != nil {
		t.Fatal(err)
	}
	ack := ref.SendPID(context.Background(), pid, pidInc{N: 3})
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("outcome=%s", ack.Outcome)
	}
	ch := make(chan int, 1)
	_ = ref.Send(context.Background(), pidGet{Ch: ch})
	select {
	case got := <-ch:
		if got != 3 {
			t.Fatalf("got=%d", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
