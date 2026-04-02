package grpctransport

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
)

// ---------------------------------------------------------------------------
// Test handler
// ---------------------------------------------------------------------------

type testHandler struct {
	mu        sync.Mutex
	envelopes []cluster.Envelope
	connected []cluster.NodeID
	lost      []cluster.NodeID
	envCh     chan cluster.Envelope
}

func newTestHandler() *testHandler {
	return &testHandler{envCh: make(chan cluster.Envelope, 64)}
}

func (h *testHandler) OnEnvelope(from cluster.NodeID, env cluster.Envelope) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.envelopes = append(h.envelopes, env)
	select {
	case h.envCh <- env:
	default:
	}
}

func (h *testHandler) OnConnectionEstablished(remote cluster.NodeID, conn cluster.Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connected = append(h.connected, remote)
}

func (h *testHandler) OnConnectionLost(remote cluster.NodeID, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lost = append(h.lost, remote)
}

func (h *testHandler) waitEnvelope(t *testing.T, timeout time.Duration) cluster.Envelope {
	t.Helper()
	select {
	case env := <-h.envCh:
		return env
	case <-time.After(timeout):
		t.Fatal("timeout waiting for envelope")
		return cluster.Envelope{}
	}
}

// ---------------------------------------------------------------------------
// Basic transport tests
// ---------------------------------------------------------------------------

func TestDialAndSend(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	env := cluster.Envelope{
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

	received := handler.waitEnvelope(t, 2*time.Second)
	if received.TargetActorID != "actor-1" {
		t.Errorf("target actor: got %s, want actor-1", received.TargetActorID)
	}
	if string(received.Payload) != "hello" {
		t.Errorf("payload: got %s, want hello", received.Payload)
	}
}

func TestRemoteNodeID(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if got := conn.RemoteNodeID(); got != "server-node" {
		t.Errorf("RemoteNodeID: got %q, want %q", got, "server-node")
	}
}

func TestMultipleEnvelopes(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	const count = 50
	for i := 0; i < count; i++ {
		if err := conn.Send(ctx, cluster.Envelope{
			SenderNode: "client-node",
			MessageID:  uint64(i),
			Payload:    []byte("msg"),
		}); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	for i := 0; i < count; i++ {
		handler.waitEnvelope(t, 2*time.Second)
	}

	handler.mu.Lock()
	got := len(handler.envelopes)
	handler.mu.Unlock()

	if got < count {
		t.Errorf("received %d envelopes, want %d", got, count)
	}
}

func TestConnectionEstablishedCallback(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send an envelope to trigger the server stream handler.
	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("ping"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	handler.waitEnvelope(t, 2*time.Second)

	handler.mu.Lock()
	connected := len(handler.connected)
	handler.mu.Unlock()

	if connected != 1 {
		t.Errorf("expected 1 connection callback, got %d", connected)
	}
}

func TestClosedTransportRejectsDial(t *testing.T) {
	tr := New("node-1")
	tr.Close()

	_, err := tr.Dial(context.Background(), "127.0.0.1:9999", nil)
	if err == nil {
		t.Fatal("expected error dialing closed transport")
	}
}

func TestAskReplyToRoundTrip(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	env := cluster.Envelope{
		SenderNode:    "client-node",
		TargetActorID: "actor-1",
		IsAsk:         true,
		AskRequestID:  "req-123",
		AskReplyTo: &cluster.RemotePID{
			Node:       "client-node",
			Namespace:  "default",
			ActorID:    "reply-actor",
			Generation: 5,
		},
		Payload: []byte("ask-msg"),
	}
	if err := conn.Send(ctx, env); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if !received.IsAsk {
		t.Error("expected IsAsk=true")
	}
	if received.AskRequestID != "req-123" {
		t.Errorf("ask request id: got %s, want req-123", received.AskRequestID)
	}
	if received.AskReplyTo == nil {
		t.Fatal("expected AskReplyTo to be non-nil")
	}
	if received.AskReplyTo.ActorID != "reply-actor" {
		t.Errorf("reply actor: got %s, want reply-actor", received.AskReplyTo.ActorID)
	}
	if received.AskReplyTo.Generation != 5 {
		t.Errorf("reply generation: got %d, want 5", received.AskReplyTo.Generation)
	}
}

// ---------------------------------------------------------------------------
// HMAC metadata auth tests
// ---------------------------------------------------------------------------

func TestHMACAuth_Success(t *testing.T) {
	ctx := context.Background()
	secret := []byte("shared-cookie")

	server := New("server-node")
	server.SetAuth(cluster.NewSharedSecretAuth(secret))
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), cluster.NewSharedSecretAuth(secret))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("auth-ok"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "auth-ok" {
		t.Errorf("payload: got %s, want auth-ok", received.Payload)
	}
}

func TestHMACAuth_WrongSecret(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	server.SetAuth(cluster.NewSharedSecretAuth([]byte("correct-secret")))
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), cluster.NewSharedSecretAuth([]byte("wrong-secret")))
	if err != nil {
		// Rejected at stream open — expected.
		return
	}
	// Stream may have opened; the send or a subsequent recv will fail.
	err = conn.Send(ctx, cluster.Envelope{SenderNode: "client-node", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error with wrong auth secret")
	}
}

func TestHMACAuth_NoCredentials(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	server.SetAuth(cluster.NewSharedSecretAuth([]byte("secret")))
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	// Dial without auth — should be rejected.
	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		return // rejected at stream open
	}
	err = conn.Send(ctx, cluster.Envelope{SenderNode: "client-node", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error dialing without auth when server requires it")
	}
}

// ---------------------------------------------------------------------------
// Header tests
// ---------------------------------------------------------------------------

func TestDefaultHeaders(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := NewWithConfig("client-node", Config{
		DefaultHeaders: map[string]string{
			HeaderTenantID: "tenant-42",
			HeaderTraceID:  "trace-abc",
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("with-headers"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "with-headers" {
		t.Errorf("payload: got %s, want with-headers", received.Payload)
	}
}

func TestSendWithHeaders(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := NewWithConfig("client-node", Config{
		DefaultHeaders: map[string]string{
			HeaderTenantID: "default-tenant",
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	hconn := conn.(HeaderConn)
	if err := hconn.SendWithHeaders(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("per-msg"),
	}, map[string]string{
		HeaderRoutingKey: "shard-7",
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "per-msg" {
		t.Errorf("payload: got %s, want per-msg", received.Payload)
	}
}

// ---------------------------------------------------------------------------
// mTLS tests
// ---------------------------------------------------------------------------

// generateTestCA creates a self-signed CA for testing.
func generateTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	return cert, key, certPEM
}

// generateTestCert creates a certificate signed by the given CA.
func generateTestCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string) (tls.Certificate, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}

	return tlsCert, certPEM
}

func TestMTLS_Success(t *testing.T) {
	ctx := context.Background()

	ca, caKey, caPEM := generateTestCA(t)
	serverCert, _ := generateTestCert(t, ca, caKey, "server-node")
	clientCert, _ := generateTestCert(t, ca, caKey, "client-node")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	server := NewWithConfig("server-node", Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
	})
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := NewWithConfig("client-node", Config{
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caPool,
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("mtls-ok"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "mtls-ok" {
		t.Errorf("payload: got %s, want mtls-ok", received.Payload)
	}
}

func TestMTLS_NoClientCert(t *testing.T) {
	ctx := context.Background()

	ca, caKey, caPEM := generateTestCA(t)
	serverCert, _ := generateTestCert(t, ca, caKey, "server-node")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	server := NewWithConfig("server-node", Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
	})
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	// Client has no cert — mTLS should reject.
	client := NewWithConfig("client-node", Config{
		ClientTLS: &tls.Config{
			RootCAs: caPool,
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		return // rejected at dial — expected
	}
	err = conn.Send(ctx, cluster.Envelope{SenderNode: "client-node", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error when client has no cert for mTLS")
	}
}

func TestMTLS_WrongCA(t *testing.T) {
	ctx := context.Background()

	ca, caKey, caPEM := generateTestCA(t)
	serverCert, _ := generateTestCert(t, ca, caKey, "server-node")

	// Client uses a cert from a different CA.
	otherCA, otherCAKey, _ := generateTestCA(t)
	clientCert, _ := generateTestCert(t, otherCA, otherCAKey, "client-node")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	server := NewWithConfig("server-node", Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
	})
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := NewWithConfig("client-node", Config{
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caPool,
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		return // rejected at TLS handshake
	}
	err = conn.Send(ctx, cluster.Envelope{SenderNode: "client-node", Payload: []byte("x")})
	if err == nil {
		t.Fatal("expected error when client cert is from wrong CA")
	}
}

func TestMTLS_WithHMACAuth(t *testing.T) {
	ctx := context.Background()
	secret := []byte("belt-and-suspenders")

	ca, caKey, caPEM := generateTestCA(t)
	serverCert, _ := generateTestCert(t, ca, caKey, "server-node")
	clientCert, _ := generateTestCert(t, ca, caKey, "client-node")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	server := NewWithConfig("server-node", Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{serverCert},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
	})
	server.SetAuth(cluster.NewSharedSecretAuth(secret))
	handler := newTestHandler()
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := NewWithConfig("client-node", Config{
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{clientCert},
			RootCAs:      caPool,
		},
	})
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), cluster.NewSharedSecretAuth(secret))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("both-auth"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	received := handler.waitEnvelope(t, 2*time.Second)
	if string(received.Payload) != "both-auth" {
		t.Errorf("payload: got %s, want both-auth", received.Payload)
	}
}

// ---------------------------------------------------------------------------
// RemoteAddr test
// ---------------------------------------------------------------------------

func TestServerConnRemoteAddr(t *testing.T) {
	ctx := context.Background()

	server := New("server-node")
	connCh := make(chan cluster.Connection, 1)
	handler := &addrCapturingHandler{
		testHandler: newTestHandler(),
		connCh:      connCh,
	}
	if err := server.Listen("127.0.0.1:0", handler); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer server.Close()

	client := New("client-node")
	defer client.Close()

	conn, err := client.Dial(ctx, server.Addr().String(), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send to trigger stream handler.
	if err := conn.Send(ctx, cluster.Envelope{
		SenderNode: "client-node",
		Payload:    []byte("ping"),
	}); err != nil {
		t.Fatalf("send: %v", err)
	}

	handler.testHandler.waitEnvelope(t, 2*time.Second)

	select {
	case srvConn := <-connCh:
		addr := srvConn.RemoteAddr()
		if addr == "" {
			t.Error("expected non-empty RemoteAddr on server-side connection")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for connection")
	}
}

type addrCapturingHandler struct {
	*testHandler
	connCh chan cluster.Connection
}

func (h *addrCapturingHandler) OnConnectionEstablished(remote cluster.NodeID, conn cluster.Connection) {
	h.testHandler.OnConnectionEstablished(remote, conn)
	select {
	case h.connCh <- conn:
	default:
	}
}
