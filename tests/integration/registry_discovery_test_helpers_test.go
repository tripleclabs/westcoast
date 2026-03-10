package integration_test

import (
	"testing"

	"github.com/tripleclabs/westcoast/src/actor"
)

func assertRegisterSuccess(t *testing.T, ack actor.RegistryRegisterAck, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("register error: %v", err)
	}
	if ack.Result != actor.RegistryRegisterSuccess {
		t.Fatalf("register result=%s", ack.Result)
	}
	if ack.PID.ActorID == "" {
		t.Fatal("register ack missing pid actor id")
	}
}

func assertLookupHit(t *testing.T, ack actor.RegistryLookupAck, wantActorID string) {
	t.Helper()
	if ack.Result != actor.RegistryLookupHit {
		t.Fatalf("lookup result=%s", ack.Result)
	}
	if ack.PID.ActorID != wantActorID {
		t.Fatalf("lookup actor_id=%s want=%s", ack.PID.ActorID, wantActorID)
	}
}

func assertLookupMiss(t *testing.T, ack actor.RegistryLookupAck) {
	t.Helper()
	if ack.Result != actor.RegistryLookupNotFound {
		t.Fatalf("lookup result=%s", ack.Result)
	}
}
