package cluster

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestDeadLetter_ActorNotFound(t *testing.T) {
	var mu sync.Mutex
	var letters []actor.DeadLetter

	rt := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithDeadLetterHandler(func(dl actor.DeadLetter) {
			mu.Lock()
			letters = append(letters, dl)
			mu.Unlock()
		}),
	)

	// Send to a PID for a non-existent actor.
	pid := actor.PID{Namespace: "node-1", ActorID: "no-such-actor", Generation: 1}
	ack := rt.SendPID(context.Background(), pid, "orphan-message")

	if ack.Outcome != actor.PIDRejectedNotFound {
		t.Fatalf("expected not found, got %s", ack.Outcome)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(letters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(letters))
	}
	if letters[0].Reason != "actor_not_found" {
		t.Errorf("reason: got %s", letters[0].Reason)
	}
	if letters[0].Payload != "orphan-message" {
		t.Errorf("payload: got %v", letters[0].Payload)
	}
}

func TestDeadLetter_ActorStopped(t *testing.T) {
	var mu sync.Mutex
	var letters []actor.DeadLetter

	rt := actor.NewRuntime(
		actor.WithNodeID("node-1"),
		actor.WithDeadLetterHandler(func(dl actor.DeadLetter) {
			mu.Lock()
			letters = append(letters, dl)
			mu.Unlock()
		}),
	)

	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}

	rt.CreateActor("svc", nil, handler)
	pid, _ := rt.IssuePID("node-1", "svc")
	time.Sleep(50 * time.Millisecond)

	rt.Stop("svc")
	time.Sleep(50 * time.Millisecond)

	ack := rt.SendPID(context.Background(), pid, "too-late")
	if ack.Outcome != actor.PIDRejectedStopped {
		t.Fatalf("expected stopped, got %s", ack.Outcome)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(letters) != 1 {
		t.Fatalf("expected 1 dead letter, got %d", len(letters))
	}
	if letters[0].Reason != "actor_stopped" {
		t.Errorf("reason: got %s", letters[0].Reason)
	}
}

func TestDeadLetter_Count(t *testing.T) {
	rt := actor.NewRuntime(actor.WithNodeID("node-1"))

	bogus := actor.PID{Namespace: "node-1", ActorID: "nope", Generation: 1}
	rt.SendPID(context.Background(), bogus, "a")
	rt.SendPID(context.Background(), bogus, "b")
	rt.SendPID(context.Background(), bogus, "c")

	if rt.DeadLetterCount() != 3 {
		t.Errorf("expected 3 dead letters, got %d", rt.DeadLetterCount())
	}
}

func TestDeadLetter_NoHandlerStillCounts(t *testing.T) {
	rt := actor.NewRuntime() // no handler set

	bogus := actor.PID{Namespace: "default", ActorID: "nope", Generation: 1}
	rt.SendPID(context.Background(), bogus, "dropped")

	if rt.DeadLetterCount() != 1 {
		t.Errorf("should count even without handler, got %d", rt.DeadLetterCount())
	}
}
