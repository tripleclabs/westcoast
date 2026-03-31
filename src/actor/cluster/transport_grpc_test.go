package cluster

import (
	"context"
	"sync"
	"testing"
	"time"
)

// testHandler captures inbound events for assertions.
type testHandler struct {
	mu        sync.Mutex
	envelopes []Envelope
	connected []NodeID
	lost      []NodeID
	envCh     chan Envelope
}

func newTestHandler() *testHandler {
	return &testHandler{envCh: make(chan Envelope, 64)}
}

func (h *testHandler) OnEnvelope(from NodeID, env Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.envelopes = append(h.envelopes, env)
	select {
	case h.envCh <- env:
	default:
	}
}

func (h *testHandler) OnConnectionEstablished(remote NodeID, conn Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connected = append(h.connected, remote)
}

func (h *testHandler) OnConnectionLost(remote NodeID, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lost = append(h.lost, remote)
}

func (h *testHandler) waitEnvelope(t *testing.T, timeout time.Duration) Envelope {
	t.Helper()
	select {
	case env := <-h.envCh:
		return env
	case <-time.After(timeout):
		t.Fatal("timeout waiting for envelope")
		return Envelope{}
	}
}

func TestTransport_DialAndSend(t *testing.T) {
	ctx := context.Background()

	serverTransport := NewGRPCTransport("server-node")
	serverHandler := newTestHandler()

	if err := serverTransport.Listen("127.0.0.1:0", serverHandler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer serverTransport.Close()

	serverAddr := serverTransport.listener.Addr().String()

	clientTransport := NewGRPCTransport("client-node")
	defer clientTransport.Close()

	conn, err := clientTransport.Dial(ctx, serverAddr, NoopAuth{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	env := Envelope{
		SenderNode:    "client-node",
		TargetNode:    "server-node",
		TargetActorID: "actor-1",
		TypeName:      "TestMsg",
		Payload:       []byte("hello"),
		MessageID:     1,
	}
	if err := conn.Send(ctx, env); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := serverHandler.waitEnvelope(t, 2*time.Second)
	if received.TargetActorID != "actor-1" {
		t.Errorf("target actor: got %s, want actor-1", received.TargetActorID)
	}
	if string(received.Payload) != "hello" {
		t.Errorf("payload: got %s, want hello", received.Payload)
	}
}

func TestTransport_AuthRejection(t *testing.T) {
	ctx := context.Background()

	serverTransport := NewGRPCTransport("server-node")
	serverTransport.SetAuth(NewSharedSecretAuth([]byte("correct-secret")))
	serverHandler := newTestHandler()

	if err := serverTransport.Listen("127.0.0.1:0", serverHandler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer serverTransport.Close()
	serverAddr := serverTransport.listener.Addr().String()

	clientTransport := NewGRPCTransport("client-node")
	defer clientTransport.Close()

	_, err := clientTransport.Dial(ctx, serverAddr, NewSharedSecretAuth([]byte("wrong-secret")))
	if err == nil {
		t.Fatal("expected error dialing with wrong auth")
	}
}

func TestTransport_AuthSuccess(t *testing.T) {
	ctx := context.Background()
	secret := []byte("shared-cookie")

	serverTransport := NewGRPCTransport("server-node")
	serverTransport.SetAuth(NewSharedSecretAuth(secret))
	serverHandler := newTestHandler()

	if err := serverTransport.Listen("127.0.0.1:0", serverHandler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer serverTransport.Close()
	serverAddr := serverTransport.listener.Addr().String()

	clientTransport := NewGRPCTransport("client-node")
	defer clientTransport.Close()

	conn, err := clientTransport.Dial(ctx, serverAddr, NewSharedSecretAuth(secret))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, Envelope{Payload: []byte("auth-ok")}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := serverHandler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "auth-ok" {
		t.Errorf("payload: got %s, want auth-ok", received.Payload)
	}
}

func TestTransport_MultipleEnvelopes(t *testing.T) {
	ctx := context.Background()

	serverTransport := NewGRPCTransport("server-node")
	serverHandler := newTestHandler()
	if err := serverTransport.Listen("127.0.0.1:0", serverHandler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer serverTransport.Close()
	serverAddr := serverTransport.listener.Addr().String()

	clientTransport := NewGRPCTransport("client-node")
	defer clientTransport.Close()

	conn, err := clientTransport.Dial(ctx, serverAddr, NoopAuth{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	const count = 50
	for i := 0; i < count; i++ {
		if err := conn.Send(ctx, Envelope{
			MessageID: uint64(i),
			Payload:   []byte("msg"),
		}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	for i := 0; i < count; i++ {
		serverHandler.waitEnvelope(t, 2*time.Second)
	}

	serverHandler.mu.Lock()
	got := len(serverHandler.envelopes)
	serverHandler.mu.Unlock()

	if got < count {
		t.Errorf("received %d envelopes, want %d", got, count)
	}
}

func TestTransport_ConnectionEstablishedCallback(t *testing.T) {
	ctx := context.Background()

	serverTransport := NewGRPCTransport("server-node")
	serverHandler := newTestHandler()
	if err := serverTransport.Listen("127.0.0.1:0", serverHandler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer serverTransport.Close()
	serverAddr := serverTransport.listener.Addr().String()

	clientTransport := NewGRPCTransport("client-node")
	defer clientTransport.Close()

	conn, err := clientTransport.Dial(ctx, serverAddr, NoopAuth{})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Give the server a moment to process the handshake.
	time.Sleep(50 * time.Millisecond)

	serverHandler.mu.Lock()
	connected := len(serverHandler.connected)
	serverHandler.mu.Unlock()

	if connected != 1 {
		t.Errorf("expected 1 connection callback, got %d", connected)
	}
}

func TestTransport_ClosedTransportRejectsDial(t *testing.T) {
	tr := NewGRPCTransport("node-1")
	tr.Close()

	_, err := tr.Dial(context.Background(), "127.0.0.1:9999", NoopAuth{})
	if err == nil {
		t.Fatal("expected error dialing closed transport")
	}
}
