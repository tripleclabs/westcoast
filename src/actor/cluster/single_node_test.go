package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

// TestSingleNode_ElectionWorks verifies that a cluster of one node
// can elect a leader, run singletons, and use the registry.
func TestSingleNode_ElectionWorks(t *testing.T) {
	election := NewRingElection("solo")
	// No other members — just self.

	leader, ok := election.Leader("my-scope")
	if !ok {
		t.Fatal("sole node should have a leader")
	}
	if leader != "solo" {
		t.Errorf("leader should be solo, got %s", leader)
	}
	if !election.IsLeader("my-scope") {
		t.Error("sole node should be leader")
	}
}

func TestSingleNode_RegistryWorks(t *testing.T) {
	registry := NewDistributedRegistry("solo")

	p := actor.PID{Namespace: "solo", ActorID: "actor-a", Generation: 1}
	if err := registry.Register("my-service", p); err != nil {
		t.Fatal(err)
	}

	got, ok := registry.Lookup("my-service")
	if !ok {
		t.Fatal("should find registered name")
	}
	if got.ActorID != "actor-a" {
		t.Errorf("got %s", got.ActorID)
	}
}

func TestSingleNode_SingletonWorks(t *testing.T) {
	election := NewRingElection("solo")
	rt := actor.NewRuntime(actor.WithNodeID("solo"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	sm := NewSingletonManager(rt, election, nil, nil, nil)
	sm.Register(SingletonSpec{Name: "my-singleton", Handler: handler})
	sm.Start(context.Background())
	defer sm.Stop()

	time.Sleep(100 * time.Millisecond)

	running := sm.Running()
	if len(running) != 1 {
		t.Fatalf("expected 1 singleton running, got %d", len(running))
	}

	// Should be able to send to the singleton by name.
	rt.RegisterName("my-singleton", "my-singleton", "solo")
	rt.IssuePID("solo", "my-singleton")

	time.Sleep(50 * time.Millisecond)
	ack := rt.SendName(context.Background(), "my-singleton", "hello")
	if ack.Outcome != actor.PIDDelivered {
		t.Errorf("expected delivered, got %s", ack.Outcome)
	}
}

func TestSingleNode_ClusterRouterWorks(t *testing.T) {
	ctx := context.Background()
	registry := NewDistributedRegistry("solo")
	rt := actor.NewRuntime(actor.WithNodeID("solo"))

	collector := newCollectingHandler()
	rt.CreateActor("worker", nil, collector.Handle)
	workerPID, _ := rt.IssuePID("solo", "worker")

	time.Sleep(50 * time.Millisecond)

	cr := NewClusterRouter(rt, registry)
	cr.Join("my-svc", workerPID)

	ack := cr.Send(ctx, "my-svc", "single-node-route")
	if ack.Outcome != actor.PIDDelivered {
		t.Fatalf("expected delivered, got %s", ack.Outcome)
	}

	msg := collector.waitMessage(t, 2*time.Second)
	if msg != "single-node-route" {
		t.Errorf("got %v", msg)
	}
}

func TestSingleNode_SendNameAndAskNameWork(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("solo"))

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		if replyTo, ok := msg.AskReplyTo(); ok {
			rt.SendPID(context.Background(), replyTo, actor.AskReplyEnvelope{
				RequestID: msg.AskRequestID(),
				Payload:   "reply:" + msg.Payload.(string),
				RepliedAt: time.Now(),
			})
		}
		return state, nil
	}

	rt.CreateActor("echo", nil, handler)
	rt.IssuePID("solo", "echo")
	rt.RegisterName("echo", "echo-svc", "solo")

	time.Sleep(50 * time.Millisecond)

	// SendName
	ack := rt.SendName(context.Background(), "echo-svc", "fire-and-forget")
	if ack.Outcome != actor.PIDDelivered {
		t.Errorf("SendName: %s", ack.Outcome)
	}

	// AskName
	result, err := rt.AskName(context.Background(), "echo-svc", "ping", 2*time.Second)
	if err != nil {
		t.Fatalf("AskName: %v", err)
	}
	if result.Payload != "reply:ping" {
		t.Errorf("AskName reply: got %v", result.Payload)
	}
}
