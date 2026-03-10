package benchmark_test

import (
	"context"
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func BenchmarkSupervisionRecoveryPath(b *testing.B) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1}))
	ref, err := rt.CreateActor("bench-super", 0, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if _, ok := msg.Payload.(struct{ Panic bool }); ok {
			panic("bench-boom")
		}
		return state, nil
	})
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ack := ref.Send(context.Background(), struct{ Panic bool }{Panic: true})
		if ack.Result != actor.SubmitAccepted {
			b.Fatalf("send=%s", ack.Result)
		}
	}
}
