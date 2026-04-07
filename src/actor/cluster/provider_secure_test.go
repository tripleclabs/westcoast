package cluster

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor"
)

func TestSecureProvider_BootstrapAndJoin(t *testing.T) {
	ctx := context.Background()
	token := "test-cluster-token"
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	// --- Bootstrap node (no seeds) ---
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:19100",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Token:   token,
			DataDir:           dir1,
			HeartbeatInterval: 200 * time.Millisecond,
		}),
	})
	if err != nil {
		t.Fatalf("Start bootstrap: %v", err)
	}
	defer c1.Stop()

	// --- Joining node ---
	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	c2, err := Start(ctx, rt2, Config{
		Addr: "127.0.0.1:19200",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Seeds:   []string{"127.0.0.1:19101"}, // join server = cluster port + 1
			Token:   token,
			DataDir:           dir2,
			HeartbeatInterval: 200 * time.Millisecond,
		}),
	})
	if err != nil {
		t.Fatalf("Start join: %v", err)
	}
	defer c2.Stop()

	// Wait for connectivity.
	deadline := time.After(10 * time.Second)
	for !c1.IsConnected("node-2") || !c2.IsConnected("node-1") {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for cluster connection")
		case <-time.After(50 * time.Millisecond):
		}
	}

	t.Log("both nodes connected via mTLS")

	// Verify singletons work across the secure cluster.
	handler := func(_ context.Context, state any, msg actor.Message) (any, error) {
		return state, nil
	}
	c1.RegisterSingleton(SingletonSpec{Name: "secure-singleton", Handler: handler})
	c2.RegisterSingleton(SingletonSpec{Name: "secure-singleton", Handler: handler})

	time.Sleep(500 * time.Millisecond)

	r1 := c1.Singletons().Running()
	r2 := c2.Singletons().Running()
	total := len(r1) + len(r2)
	if total != 1 {
		t.Errorf("expected 1 singleton total, got node-1=%v node-2=%v", r1, r2)
	}
}

func TestSecureProvider_WrongToken(t *testing.T) {
	ctx := context.Background()
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:19300",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Token:   "correct-token",
			DataDir:           dir1,
			HeartbeatInterval: 200 * time.Millisecond,
		}),
	})
	if err != nil {
		t.Fatalf("Start bootstrap: %v", err)
	}
	defer c1.Stop()

	rt2 := actor.NewRuntime(actor.WithNodeID("intruder"))
	_, err = Start(ctx, rt2, Config{
		Addr: "127.0.0.1:19400",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Seeds:   []string{"127.0.0.1:19301"},
			Token:   "wrong-token",
			DataDir:           dir2,
			HeartbeatInterval: 200 * time.Millisecond,
		}),
	})
	if err == nil {
		t.Fatal("expected error joining with wrong token")
	}
	t.Logf("correctly rejected: %v", err)
}

func TestSecureProvider_CertPersistence(t *testing.T) {
	ctx := context.Background()
	token := "persist-token"
	dir := t.TempDir()

	// First boot — generates certs.
	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:19500",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Token:   token,
			DataDir: dir,
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	c1.Stop()

	// Second boot — loads persisted certs (no join handshake needed).
	rt2 := actor.NewRuntime(actor.WithNodeID("node-1"))
	c2, err := Start(ctx, rt2, Config{
		Addr: "127.0.0.1:19500",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Token:   token,
			DataDir: dir,
		}),
	})
	if err != nil {
		t.Fatalf("restart with persisted certs failed: %v", err)
	}
	defer c2.Stop()
}

func TestSecureProvider_OnJoinPayload(t *testing.T) {
	ctx := context.Background()
	token := "payload-token"
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	var receivedPayload atomic.Value

	rt1 := actor.NewRuntime(actor.WithNodeID("node-1"))
	c1, err := Start(ctx, rt1, Config{
		Addr: "127.0.0.1:19600",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Token:   token,
			DataDir: dir1,
			OnJoin: func(joiningNodeID NodeID) ([]byte, error) {
				return []byte("secret-for-" + string(joiningNodeID)), nil
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c1.Stop()

	rt2 := actor.NewRuntime(actor.WithNodeID("node-2"))
	c2, err := Start(ctx, rt2, Config{
		Addr: "127.0.0.1:19700",
		Provider: NewSecureFixedProvider(SecureFixedProviderConfig{
			Seeds:   []string{"127.0.0.1:19601"},
			Token:   token,
			DataDir: dir2,
			OnJoined: func(payload []byte) error {
				receivedPayload.Store(string(payload))
				return nil
			},
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Stop()

	v := receivedPayload.Load()
	if v == nil || v.(string) != "secret-for-node-2" {
		t.Errorf("expected 'secret-for-node-2', got %v", v)
	}
}
