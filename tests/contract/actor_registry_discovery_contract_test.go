package contract_test

import (
	"context"
	"testing"
	"time"

	"westcoast/src/actor"
)

func TestRegistryOperationOutcomeSet(t *testing.T) {
	vals := []actor.RegistryOperationResult{
		actor.RegistryRegisterSuccess,
		actor.RegistryRegisterRejectedDup,
		actor.RegistryLookupHit,
		actor.RegistryLookupNotFound,
		actor.RegistryUnregisterSuccess,
		actor.RegistryUnregisterLifecycleTerm,
	}
	for _, v := range vals {
		if string(v) == "" {
			t.Fatal("empty registry result")
		}
	}
}

func TestRegistryRegistrationValidationContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("reg-c", 0, noop)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ref.RegisterName(""); err == nil {
		t.Fatal("expected invalid name error")
	}
}

func TestRegistryLookupHitMissContract(t *testing.T) {
	rt := actor.NewRuntime()
	ref, err := rt.CreateActor("reg-l", 0, noop)
	if err != nil {
		t.Fatal(err)
	}
	ack, err := ref.RegisterName("svc-contract")
	if err != nil {
		t.Fatal(err)
	}
	if ack.Result != actor.RegistryRegisterSuccess {
		t.Fatalf("register result=%s", ack.Result)
	}
	hit := rt.LookupName("svc-contract")
	if hit.Result != actor.RegistryLookupHit {
		t.Fatalf("hit=%s", hit.Result)
	}
	miss := rt.LookupName("svc-contract-missing")
	if miss.Result != actor.RegistryLookupNotFound {
		t.Fatalf("miss=%s", miss.Result)
	}
}

func TestRegistryLifecycleTerminalUnregisterContract(t *testing.T) {
	rt := actor.NewRuntime(actor.WithSupervisor(actor.DefaultSupervisor{MaxRestarts: 0, OnLimit: actor.DecisionStop}))
	ref, err := rt.CreateActor("reg-life", 0, func(_ context.Context, state any, msg actor.Message) (any, error) {
		if msg.Payload == "panic" {
			panic("boom")
		}
		return state, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ref.RegisterName("svc-life"); err != nil {
		t.Fatal(err)
	}
	_ = ref.Send(context.Background(), "panic")
	waitForContractStatus(t, ref, actor.ActorStopped)
	if got := rt.LookupName("svc-life"); got.Result != actor.RegistryLookupNotFound {
		t.Fatalf("lookup after terminal=%s", got.Result)
	}
}

func waitForContractStatus(t *testing.T, ref *actor.ActorRef, want actor.ActorStatus) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if ref.Status() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status=%s want=%s", ref.Status(), want)
}
