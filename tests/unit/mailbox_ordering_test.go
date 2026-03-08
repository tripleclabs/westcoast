package unit_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"westcoast/src/actor"
)

type appendNum struct{ N int }
type getList struct{ Ch chan []int }

func listHandler(_ context.Context, state any, msg actor.Message) (any, error) {
	cur := state.([]int)
	switch p := msg.Payload.(type) {
	case appendNum:
		cur = append(cur, p.N)
		return cur, nil
	case getList:
		copyCur := append([]int(nil), cur...)
		p.Ch <- copyCur
		return cur, nil
	default:
		return cur, nil
	}
}

func TestMailboxOrdering(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("ord", []int{}, listHandler)
	if err != nil {
		t.Fatalf("create actor: %v", err)
	}

	for i := 1; i <= 20; i++ {
		if r := ref.Send(context.Background(), appendNum{N: i}).Result; r != actor.SubmitAccepted {
			t.Fatalf("send %d result = %s", i, r)
		}
	}

	ch := make(chan []int, 1)
	ref.Send(context.Background(), getList{Ch: ch})
	select {
	case got := <-ch:
		want := make([]int, 20)
		for i := range want {
			want[i] = i + 1
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got order %v, want %v", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for order")
	}
}
