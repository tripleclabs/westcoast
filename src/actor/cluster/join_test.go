package cluster

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestJoin_SuccessfulHandshake(t *testing.T) {
	token := "test-cluster-token"

	handler := func(req JoinRequest) (*JoinResponse, error) {
		return &JoinResponse{
			NodeCertDER: []byte("signed-cert-for-" + req.NodeID),
			CACertDER:   []byte("ca-cert"),
			CAKeyPKCS8:  []byte("ca-key"),
			ServerID:    "bootstrap-node",
			ClusterAddr: "10.0.0.1:9000",
			Payload:     []byte("app-secret"),
		}, nil
	}

	js, err := NewJoinServer("127.0.0.1:0", token, handler)
	if err != nil {
		t.Fatal(err)
	}
	defer js.Close()
	go js.Serve()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := JoinClient(ctx, js.Addr(), token, JoinRequest{
		NodeID: "node-2",
		CSRDER: []byte("csr-data"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if string(resp.NodeCertDER) != "signed-cert-for-node-2" {
		t.Errorf("cert = %q", resp.NodeCertDER)
	}
	if string(resp.CACertDER) != "ca-cert" {
		t.Errorf("CA cert = %q", resp.CACertDER)
	}
	if string(resp.CAKeyPKCS8) != "ca-key" {
		t.Errorf("CA key = %q", resp.CAKeyPKCS8)
	}
	if resp.ServerID != "bootstrap-node" {
		t.Errorf("server ID = %q", resp.ServerID)
	}
	if string(resp.Payload) != "app-secret" {
		t.Errorf("payload = %q", resp.Payload)
	}
}

func TestJoin_WrongToken(t *testing.T) {
	handler := func(req JoinRequest) (*JoinResponse, error) {
		t.Fatal("handler should not be called with wrong token")
		return nil, nil
	}

	js, err := NewJoinServer("127.0.0.1:0", "correct-token", handler)
	if err != nil {
		t.Fatal(err)
	}
	defer js.Close()
	go js.Serve()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = JoinClient(ctx, js.Addr(), "wrong-token", JoinRequest{
		NodeID: "intruder",
		CSRDER: []byte("bad-csr"),
	})
	if err == nil {
		t.Fatal("expected error with wrong token")
	}
}

func TestJoin_NoPayload(t *testing.T) {
	handler := func(req JoinRequest) (*JoinResponse, error) {
		return &JoinResponse{
			NodeCertDER: []byte("cert"),
			CACertDER:   []byte("ca"),
			CAKeyPKCS8:  []byte("key"),
			ServerID:    "node-1",
			ClusterAddr: "10.0.0.1:9000",
			Payload:     nil, // no payload
		}, nil
	}

	js, err := NewJoinServer("127.0.0.1:0", "token", handler)
	if err != nil {
		t.Fatal(err)
	}
	defer js.Close()
	go js.Serve()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := JoinClient(ctx, js.Addr(), "token", JoinRequest{
		NodeID: "node-2",
		CSRDER: []byte("csr"),
	})
	if err != nil {
		t.Fatal(err)
	}

	if resp.Payload != nil {
		t.Errorf("expected nil payload, got %q", resp.Payload)
	}
}

func TestJoin_MultipleClients(t *testing.T) {
	var joined []string
	handler := func(req JoinRequest) (*JoinResponse, error) {
		joined = append(joined, req.NodeID)
		return &JoinResponse{
			NodeCertDER: []byte("cert"),
			CACertDER:   []byte("ca"),
			CAKeyPKCS8:  []byte("key"),
			ServerID:    "node-1",
			ClusterAddr: "10.0.0.1:9000",
		}, nil
	}

	js, err := NewJoinServer("127.0.0.1:0", "token", handler)
	if err != nil {
		t.Fatal(err)
	}
	defer js.Close()
	go js.Serve()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := range 5 {
		nodeID := fmt.Sprintf("node-%d", i+2)
		_, err := JoinClient(ctx, js.Addr(), "token", JoinRequest{
			NodeID: nodeID,
			CSRDER: []byte("csr"),
		})
		if err != nil {
			t.Fatalf("join %s: %v", nodeID, err)
		}
	}

	if len(joined) != 5 {
		t.Errorf("expected 5 joins, got %d", len(joined))
	}
}
