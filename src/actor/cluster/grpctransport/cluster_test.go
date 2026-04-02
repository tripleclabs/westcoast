package grpctransport_test

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
	"testing"
	"time"

	"github.com/tripleclabs/westcoast/src/actor/cluster"
	"github.com/tripleclabs/westcoast/src/actor/cluster/grpctransport"
)

// waitConnected polls until c.IsConnected(target) returns true or times out.
func waitConnected(t *testing.T, c *cluster.Cluster, target cluster.NodeID) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !c.IsConnected(target) {
		select {
		case <-deadline:
			t.Fatalf("timeout waiting for connection to %s", target)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// ---------------------------------------------------------------------------
// Two-node cluster formation + message delivery (no auth, plaintext)
// ---------------------------------------------------------------------------

func TestCluster_GRPCTransport_TwoNodeFormation(t *testing.T) {
	ctx := context.Background()
	received := make(chan cluster.Envelope, 64)

	transport1 := grpctransport.New("node-1")
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.New("node-2")
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")

	env := cluster.Envelope{
		SenderNode:    "node-1",
		SenderActorID: "actor-a",
		TargetNode:    "node-2",
		TargetActorID: "actor-b",
		MessageID:     42,
		Payload:       []byte("hello over grpc"),
	}
	if err := c1.SendRemote(ctx, "node-2", env); err != nil {
		t.Fatalf("send remote: %v", err)
	}

	select {
	case got := <-received:
		if got.TargetActorID != "actor-b" {
			t.Errorf("target actor: got %s, want actor-b", got.TargetActorID)
		}
		if string(got.Payload) != "hello over grpc" {
			t.Errorf("payload: got %s", got.Payload)
		}
		if got.MessageID != 42 {
			t.Errorf("message ID: got %d, want 42", got.MessageID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for envelope delivery")
	}
}

// ---------------------------------------------------------------------------
// Two-node cluster with HMAC shared secret auth
// ---------------------------------------------------------------------------

func TestCluster_GRPCTransport_SharedSecretAuth(t *testing.T) {
	ctx := context.Background()
	secret := []byte("cluster-cookie-42")
	received := make(chan cluster.Envelope, 64)

	transport1 := grpctransport.New("node-1")
	transport1.SetAuth(cluster.NewSharedSecretAuth(secret))
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NewSharedSecretAuth(secret),
		Codec:     cluster.NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.New("node-2")
	transport2.SetAuth(cluster.NewSharedSecretAuth(secret))
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NewSharedSecretAuth(secret),
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")

	env := cluster.Envelope{
		SenderNode:    "node-1",
		TargetNode:    "node-2",
		TargetActorID: "actor-secret",
		Payload:       []byte("authenticated message"),
	}
	if err := c1.SendRemote(ctx, "node-2", env); err != nil {
		t.Fatalf("send remote: %v", err)
	}

	select {
	case got := <-received:
		if string(got.Payload) != "authenticated message" {
			t.Errorf("payload: got %s", got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for authenticated envelope delivery")
	}
}

// ---------------------------------------------------------------------------
// Two-node cluster with mTLS
// ---------------------------------------------------------------------------

func generateCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "cluster-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	return cert, key, pemBytes
}

func generateCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn string) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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
	der, _ := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	keyDER, _ := x509.MarshalECPrivateKey(key)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	tlsCert, _ := tls.X509KeyPair(certPEM, keyPEM)
	return tlsCert
}

func TestCluster_GRPCTransport_MTLS(t *testing.T) {
	ctx := context.Background()
	received := make(chan cluster.Envelope, 64)

	ca, caKey, caPEM := generateCA(t)
	cert1 := generateCert(t, ca, caKey, "node-1")
	cert2 := generateCert(t, ca, caKey, "node-2")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	transport1 := grpctransport.NewWithConfig("node-1", grpctransport.Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{cert1},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{cert1},
			RootCAs:      caPool,
		},
	})
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.NewWithConfig("node-2", grpctransport.Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{cert2},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{cert2},
			RootCAs:      caPool,
		},
	})
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")

	env := cluster.Envelope{
		SenderNode:    "node-1",
		TargetNode:    "node-2",
		TargetActorID: "actor-mtls",
		Payload:       []byte("mtls cluster message"),
	}
	if err := c1.SendRemote(ctx, "node-2", env); err != nil {
		t.Fatalf("send remote: %v", err)
	}

	select {
	case got := <-received:
		if string(got.Payload) != "mtls cluster message" {
			t.Errorf("payload: got %s", got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for mTLS envelope delivery")
	}
}

// ---------------------------------------------------------------------------
// Two-node cluster with mTLS + HMAC (belt and suspenders)
// ---------------------------------------------------------------------------

func TestCluster_GRPCTransport_MTLS_WithHMAC(t *testing.T) {
	ctx := context.Background()
	secret := []byte("double-auth-secret")
	received := make(chan cluster.Envelope, 64)

	ca, caKey, caPEM := generateCA(t)
	cert1 := generateCert(t, ca, caKey, "node-1")
	cert2 := generateCert(t, ca, caKey, "node-2")

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(caPEM)

	transport1 := grpctransport.NewWithConfig("node-1", grpctransport.Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{cert1},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{cert1},
			RootCAs:      caPool,
		},
	})
	transport1.SetAuth(cluster.NewSharedSecretAuth(secret))
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NewSharedSecretAuth(secret),
		Codec:     cluster.NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.NewWithConfig("node-2", grpctransport.Config{
		ServerTLS: &tls.Config{
			Certificates: []tls.Certificate{cert2},
			ClientCAs:    caPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		},
		ClientTLS: &tls.Config{
			Certificates: []tls.Certificate{cert2},
			RootCAs:      caPool,
		},
	})
	transport2.SetAuth(cluster.NewSharedSecretAuth(secret))
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NewSharedSecretAuth(secret),
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")

	env := cluster.Envelope{
		SenderNode:    "node-1",
		TargetNode:    "node-2",
		TargetActorID: "actor-belt-suspenders",
		Payload:       []byte("double auth message"),
	}
	if err := c1.SendRemote(ctx, "node-2", env); err != nil {
		t.Fatalf("send remote: %v", err)
	}

	select {
	case got := <-received:
		if string(got.Payload) != "double auth message" {
			t.Errorf("payload: got %s", got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for mTLS+HMAC envelope delivery")
	}
}

// ---------------------------------------------------------------------------
// Multiple envelopes over gRPC cluster
// ---------------------------------------------------------------------------

func TestCluster_GRPCTransport_MultipleEnvelopes(t *testing.T) {
	ctx := context.Background()
	received := make(chan cluster.Envelope, 128)

	transport1 := grpctransport.New("node-1")
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.New("node-2")
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")

	const count = 50
	for i := 0; i < count; i++ {
		env := cluster.Envelope{
			SenderNode:    "node-1",
			TargetNode:    "node-2",
			TargetActorID: "actor-bulk",
			MessageID:     uint64(i),
			Payload:       []byte("bulk"),
		}
		if err := c1.SendRemote(ctx, "node-2", env); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	for i := 0; i < count; i++ {
		select {
		case <-received:
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout at envelope %d/%d", i, count)
		}
	}
}

// ---------------------------------------------------------------------------
// Bidirectional: both nodes send to each other
// ---------------------------------------------------------------------------

func TestCluster_GRPCTransport_Bidirectional(t *testing.T) {
	ctx := context.Background()
	received1 := make(chan cluster.Envelope, 64)
	received2 := make(chan cluster.Envelope, 64)

	transport1 := grpctransport.New("node-1")
	provider1 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c1, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-1", Addr: "127.0.0.1:0"},
		Provider:  provider1,
		Transport: transport1,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received1 <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster1: %v", err)
	}
	if err := c1.Start(ctx); err != nil {
		t.Fatalf("start cluster1: %v", err)
	}
	defer c1.Stop()

	addr1 := transport1.Addr().String()

	transport2 := grpctransport.New("node-2")
	provider2 := cluster.NewFixedProvider(cluster.FixedProviderConfig{
		HeartbeatInterval: 100 * time.Millisecond,
	})
	c2, err := cluster.NewCluster(cluster.ClusterConfig{
		Self:      cluster.NodeMeta{ID: "node-2", Addr: "127.0.0.1:0"},
		Provider:  provider2,
		Transport: transport2,
		Auth:      cluster.NoopAuth{},
		Codec:     cluster.NewGobCodec(),
		OnEnvelope: func(from cluster.NodeID, env cluster.Envelope) {
			received2 <- env
		},
	})
	if err != nil {
		t.Fatalf("new cluster2: %v", err)
	}
	if err := c2.Start(ctx); err != nil {
		t.Fatalf("start cluster2: %v", err)
	}
	defer c2.Stop()

	addr2 := transport2.Addr().String()

	provider1.AddMember(cluster.NodeMeta{ID: "node-2", Addr: addr2})
	provider2.AddMember(cluster.NodeMeta{ID: "node-1", Addr: addr1})

	waitConnected(t, c1, "node-2")
	waitConnected(t, c2, "node-1")

	// Node-1 -> Node-2
	if err := c1.SendRemote(ctx, "node-2", cluster.Envelope{
		SenderNode: "node-1", TargetNode: "node-2",
		Payload: []byte("from-1"),
	}); err != nil {
		t.Fatalf("send 1->2: %v", err)
	}

	// Node-2 -> Node-1
	if err := c2.SendRemote(ctx, "node-1", cluster.Envelope{
		SenderNode: "node-2", TargetNode: "node-1",
		Payload: []byte("from-2"),
	}); err != nil {
		t.Fatalf("send 2->1: %v", err)
	}

	select {
	case got := <-received2:
		if string(got.Payload) != "from-1" {
			t.Errorf("node-2 received: got %s, want from-1", got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for node-2 to receive")
	}

	select {
	case got := <-received1:
		if string(got.Payload) != "from-2" {
			t.Errorf("node-1 received: got %s, want from-2", got.Payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for node-1 to receive")
	}
}
