package cluster

import (
	"context"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/tripleclabs/westcoast/pkg/certmanager"
)

// SecureFixedProviderConfig configures a secure cluster bootstrap provider.
type SecureFixedProviderConfig struct {
	// Seeds are join server addresses of existing cluster nodes (typically
	// the cluster address with port+1). If empty, this node is the
	// bootstrap node and generates the cluster CA.
	Seeds []string

	// Token is the shared cluster token used for HMAC authentication
	// during the join handshake. All nodes in the cluster must use the
	// same token. The token never crosses the wire.
	Token string

	// DataDir is where certificates are persisted. On restart, existing
	// certs are loaded from DataDir/certs/ instead of re-joining.
	DataDir string

	// JoinAddr is the address for the join server. Defaults to the
	// cluster address with port incremented by 1.
	JoinAddr string

	// HeartbeatInterval for the underlying FixedProvider. Default 5s.
	HeartbeatInterval time.Duration

	// OnJoin is called on the accepting node when a new node joins.
	// Return app-level payload to send to the joining node.
	OnJoin func(joiningNodeID NodeID) ([]byte, error)

	// OnJoined is called on the joining node after receiving certs
	// and the optional payload from the accepting node.
	OnJoined func(payload []byte) error
}

// SecureFixedProvider wraps FixedProvider with automatic mTLS bootstrap.
// It implements BootstrappingProvider — cluster.Start() calls Bootstrap()
// before starting the transport.
//
// Bootstrap node (no seeds): generates CA, starts join server.
// Joining node (has seeds): performs HMAC join handshake, receives certs.
// All nodes: start join server after obtaining CA key, enabling any node
// to accept future joins (no single point of failure).
type SecureFixedProvider struct {
	cfg   SecureFixedProviderConfig
	inner *FixedProvider

	ca         *certmanager.CA
	node       *certmanager.NodeCert
	js         *JoinServer
	joinedSeed *NodeMeta // seed discovered during join handshake
}

// NewSecureFixedProvider creates a secure provider. Call Bootstrap() (or
// let cluster.Start() call it) before Start().
func NewSecureFixedProvider(cfg SecureFixedProviderConfig) *SecureFixedProvider {
	return &SecureFixedProvider{cfg: cfg}
}

// Bootstrap implements BootstrappingProvider. It handles cert generation
// or join handshake, starts the join server, and returns a TLS-configured
// transport and CertAuth.
func (sp *SecureFixedProvider) Bootstrap(ctx context.Context, self NodeMeta) (Transport, ClusterAuth, error) {
	certDir := sp.cfg.DataDir + "/certs"

	// Try loading existing certs first.
	if certmanager.CertsExist(certDir) {
		ca, node, err := certmanager.LoadCerts(certDir)
		if err != nil {
			return nil, nil, fmt.Errorf("secure provider: load certs: %w", err)
		}
		sp.ca = ca
		sp.node = node
	} else if len(sp.cfg.Seeds) == 0 {
		// Bootstrap node: generate CA and self-signed node cert.
		ca, err := certmanager.GenerateCA()
		if err != nil {
			return nil, nil, fmt.Errorf("secure provider: generate CA: %w", err)
		}
		node, err := certmanager.GenerateNodeCert(ca, string(self.ID), self.Addr)
		if err != nil {
			return nil, nil, fmt.Errorf("secure provider: generate node cert: %w", err)
		}
		sp.ca = ca
		sp.node = node

		if err := certmanager.SaveCerts(certDir, ca, node); err != nil {
			return nil, nil, fmt.Errorf("secure provider: save certs: %w", err)
		}
	} else {
		// Joining node: perform join handshake with a seed.
		if err := sp.joinCluster(ctx, self); err != nil {
			return nil, nil, fmt.Errorf("secure provider: join: %w", err)
		}
		if err := certmanager.SaveCerts(certDir, sp.ca, sp.node); err != nil {
			return nil, nil, fmt.Errorf("secure provider: save certs: %w", err)
		}
	}

	// Start join server so this node can accept future joins.
	joinAddr := sp.joinAddr(self.Addr)
	if err := sp.startJoinServer(joinAddr, self.ID, self.Addr); err != nil {
		return nil, nil, fmt.Errorf("secure provider: join server: %w", err)
	}

	// Create inner FixedProvider.
	sp.inner = NewFixedProvider(FixedProviderConfig{
		HeartbeatInterval: sp.cfg.HeartbeatInterval,
	})
	if sp.joinedSeed != nil {
		sp.inner.AddSeed(*sp.joinedSeed)
	}

	// Configure mTLS transport.
	tlsCfg := certmanager.ServerTLSConfig(sp.ca, sp.node)
	// Use the same TLS config for both server and client side.
	// The client TLS config allows connecting to any node signed by our CA.
	clientTLS := certmanager.ClientTLSConfig(sp.ca, sp.node)
	// Merge: server needs ClientAuth, client needs RootCAs.
	tlsCfg.RootCAs = clientTLS.RootCAs

	if defaultTransportFactory == nil {
		return nil, nil, fmt.Errorf("no transport factory registered (import grpctransport)")
	}
	transport := defaultTransportFactory(self.ID, tlsCfg)
	auth := NewCertAuth(sp.ca.Cert, sp.node.Cert)

	return transport, auth, nil
}

// Start implements ClusterProvider by delegating to the inner FixedProvider.
func (sp *SecureFixedProvider) Start(self NodeMeta) error {
	return sp.inner.Start(self)
}

// Stop implements ClusterProvider.
func (sp *SecureFixedProvider) Stop() error {
	if sp.js != nil {
		sp.js.Close()
	}
	return sp.inner.Stop()
}

// Members implements ClusterProvider.
func (sp *SecureFixedProvider) Members() []NodeMeta {
	return sp.inner.Members()
}

// Events implements ClusterProvider.
func (sp *SecureFixedProvider) Events() <-chan MemberEvent {
	return sp.inner.Events()
}

// HasSeeds implements SeedProvider.
func (sp *SecureFixedProvider) HasSeeds() bool {
	return len(sp.cfg.Seeds) > 0
}

// AddSeed exposes the inner provider's AddSeed for manual seed addition.
func (sp *SecureFixedProvider) AddSeed(meta NodeMeta) {
	sp.inner.AddSeed(meta)
}

// JoinServerAddr returns the join server's listen address, or "" if not
// started. Useful for tests and dynamic port assignment.
func (sp *SecureFixedProvider) JoinServerAddr() string {
	if sp.js == nil {
		return ""
	}
	return sp.js.Addr()
}

// joinCluster performs the join handshake with seeds.
func (sp *SecureFixedProvider) joinCluster(ctx context.Context, self NodeMeta) error {
	csrDER, nodeKey, err := certmanager.GenerateCSR(string(self.ID))
	if err != nil {
		return err
	}

	var lastErr error
	for _, seed := range sp.cfg.Seeds {
		resp, err := JoinClient(ctx, seed, sp.cfg.Token, JoinRequest{
			NodeID:      string(self.ID),
			ClusterAddr: self.Addr,
			CSRDER:      csrDER,
		})
		if err != nil {
			lastErr = err
			continue
		}

		// Parse received certs.
		caCert, err := x509.ParseCertificate(resp.CACertDER)
		if err != nil {
			lastErr = fmt.Errorf("parse CA cert: %w", err)
			continue
		}
		nodeCert, err := x509.ParseCertificate(resp.NodeCertDER)
		if err != nil {
			lastErr = fmt.Errorf("parse node cert: %w", err)
			continue
		}

		caKeyParsed, err := certmanager.ParseKey(resp.CAKeyPKCS8)
		if err != nil {
			lastErr = fmt.Errorf("parse CA key: %w", err)
			continue
		}
		sp.ca = &certmanager.CA{
			Cert:    caCert,
			CertDER: resp.CACertDER,
			Key:     caKeyParsed,
		}

		sp.node = &certmanager.NodeCert{
			Cert:    nodeCert,
			CertDER: resp.NodeCertDER,
			Key:     nodeKey,
		}

		// Store the seed for the inner FixedProvider (created in Bootstrap after this returns).
		seed := NodeMeta{ID: NodeID(resp.ServerID), Addr: resp.ClusterAddr}
		sp.joinedSeed = &seed

		// Call OnJoined callback if set.
		if sp.cfg.OnJoined != nil && resp.Payload != nil {
			if err := sp.cfg.OnJoined(resp.Payload); err != nil {
				return fmt.Errorf("OnJoined callback: %w", err)
			}
		}

		return nil
	}
	return fmt.Errorf("failed to join any seed: %w", lastErr)
}

// startJoinServer starts the join server for accepting new nodes.
// clusterAddr is the cluster transport listen address (sent to joining
// nodes so they can add it as a FixedProvider seed).
func (sp *SecureFixedProvider) startJoinServer(addr string, selfID NodeID, configAddr string) error {
	handler := func(req JoinRequest) (*JoinResponse, error) {
		clusterAddr := configAddr
		// Sign the joining node's CSR.
		certDER, err := certmanager.SignCSR(sp.ca, req.CSRDER, req.ClusterAddr)
		if err != nil {
			return nil, fmt.Errorf("sign CSR: %w", err)
		}

		// Marshal CA key for transmission.
		caKeyPKCS8, err := x509.MarshalPKCS8PrivateKey(sp.ca.Key)
		if err != nil {
			return nil, fmt.Errorf("marshal CA key: %w", err)
		}

		// Get app-level payload.
		var payload []byte
		if sp.cfg.OnJoin != nil {
			payload, err = sp.cfg.OnJoin(NodeID(req.NodeID))
			if err != nil {
				return nil, fmt.Errorf("OnJoin callback: %w", err)
			}
		}

		// Add joining node as a seed so we start probing it.
		if sp.inner != nil {
			sp.inner.AddSeed(NodeMeta{ID: NodeID(req.NodeID), Addr: req.ClusterAddr})
		}

		return &JoinResponse{
			NodeCertDER: certDER,
			CACertDER:   sp.ca.CertDER,
			CAKeyPKCS8:  caKeyPKCS8,
			ServerID:    string(selfID),
			ClusterAddr: clusterAddr,
			Payload:     payload,
		}, nil
	}

	js, err := NewJoinServer(addr, sp.cfg.Token, handler)
	if err != nil {
		return err
	}
	sp.js = js
	go js.Serve()
	return nil
}

func (sp *SecureFixedProvider) joinAddr(clusterAddr string) string {
	if sp.cfg.JoinAddr != "" {
		return sp.cfg.JoinAddr
	}
	return incrementPort(clusterAddr)
}

// incrementPort takes a host:port string and returns host:(port+1).
// If port is 0 (random), returns host:0 (also random).
func incrementPort(addr string) string {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return addr
	}
	if port == 0 {
		return net.JoinHostPort(host, "0")
	}
	return net.JoinHostPort(host, strconv.Itoa(port+1))
}

