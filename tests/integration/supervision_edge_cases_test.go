package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestRestartStormStopsAtLimit(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1, OnLimit: actor.DecisionStop}))
	ref, err := rt.CreateActor("storm", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	_ = ref.Send(context.Background(), panicMsg{})
	_ = ref.Send(context.Background(), panicMsg{})
	_ = ref.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorStopped }, "expected actor to stop after restart storm")
}

func TestStopPolicyRejectsFurtherMailboxTraffic(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 1, OnLimit: actor.DecisionStop}))
	ref, err := rt.CreateActor("stop-mb", 0, panicHandler)
	if err != nil {
		t.Fatal(err)
	}
	_ = ref.Send(context.Background(), panicMsg{})
	_ = ref.Send(context.Background(), panicMsg{})
	waitFor(t, time.Second, func() bool { return ref.Status() == actor.ActorStopped }, "expected actor to stop before follow-up send")
	ack := ref.Send(context.Background(), inc{N: 1})
	if ack.Result != actor.SubmitRejectedStop {
		t.Fatalf("ack=%s want=%s", ack.Result, actor.SubmitRejectedStop)
	}
}
