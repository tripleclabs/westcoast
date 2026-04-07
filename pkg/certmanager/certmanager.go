// Package certmanager provides CA and node certificate generation, CSR
// signing, TLS configuration, and cert persistence for cluster mTLS.
// It uses ED25519 keys throughout for fast key generation and small certs.
//
// This package is provider-agnostic — it can be used by SecureFixedProvider,
// a future secure multicast provider, or any other bootstrap mechanism
// that needs cluster-internal PKI.
package certmanager

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CA holds a cluster certificate authority.
type CA struct {
	Cert    *x509.Certificate
	CertDER []byte
	Key     ed25519.PrivateKey
}

// NodeCert holds a node's certificate and private key.
type NodeCert struct {
	Cert    *x509.Certificate
	CertDER []byte
	Key     ed25519.PrivateKey
}

// GenerateCA creates a new self-signed ED25519 certificate authority.
// The CA cert is valid for 10 years.
func GenerateCA() (*CA, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("certmanager: generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "westcoast-cluster-ca"},
		NotBefore:    now,
		NotAfter:     now.Add(10 * 365 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, pub, priv)
	if err != nil {
		return nil, fmt.Errorf("certmanager: create CA cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("certmanager: parse CA cert: %w", err)
	}

	return &CA{Cert: cert, CertDER: certDER, Key: priv}, nil
}

// GenerateCSR creates a certificate signing request and a new ED25519 key
// for a node. The CSR includes the node ID as the Common Name.
func GenerateCSR(nodeID string) (csrDER []byte, key ed25519.PrivateKey, err error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: generate node key: %w", err)
	}

	template := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: nodeID},
	}

	csrDER, err = x509.CreateCertificateRequest(rand.Reader, template, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: create CSR: %w", err)
	}

	_ = pub // used via priv.Public()
	return csrDER, priv, nil
}

// SignCSR signs a certificate signing request with the CA, producing a
// node certificate. The cert is valid for 1 year and includes IP SANs
// parsed from the node's address.
func SignCSR(ca *CA, csrDER []byte, addr string) (certDER []byte, err error) {
	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, fmt.Errorf("certmanager: parse CSR: %w", err)
	}
	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("certmanager: CSR signature invalid: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      csr.Subject,
		NotBefore:    now,
		NotAfter:     now.Add(365 * 24 * time.Hour),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	// Add IP SANs from the address.
	if host, _, err := net.SplitHostPort(addr); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = []net.IP{ip}
		}
	}

	certDER, err = x509.CreateCertificate(rand.Reader, template, ca.Cert, csr.PublicKey, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("certmanager: sign cert: %w", err)
	}
	return certDER, nil
}

// GenerateNodeCert creates a signed node certificate directly (without CSR).
// Convenience for the bootstrap node that signs its own cert.
func GenerateNodeCert(ca *CA, nodeID, addr string) (*NodeCert, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("certmanager: generate node key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: nodeID},
		NotBefore:    now,
		NotAfter:     now.Add(365 * 24 * time.Hour),

		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	if host, _, err := net.SplitHostPort(addr); err == nil {
		if ip := net.ParseIP(host); ip != nil {
			template.IPAddresses = []net.IP{ip}
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca.Cert, pub, ca.Key)
	if err != nil {
		return nil, fmt.Errorf("certmanager: sign node cert: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("certmanager: parse node cert: %w", err)
	}

	return &NodeCert{Cert: cert, CertDER: certDER, Key: priv}, nil
}

// ServerTLSConfig returns a TLS config for a cluster server (listener).
// Requires client certificates signed by the cluster CA (mTLS).
func ServerTLSConfig(ca *CA, node *NodeCert) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{node.CertDER},
			PrivateKey:  node.Key,
			Leaf:        node.Cert,
		}},
		ClientCAs:  pool,
		ClientAuth: tls.RequireAndVerifyClientCert,
		MinVersion: tls.VersionTLS13,
	}
}

// ClientTLSConfig returns a TLS config for a cluster client (dialer).
// Presents the node certificate and verifies the server's cert against the CA.
func ClientTLSConfig(ca *CA, node *NodeCert) *tls.Config {
	pool := x509.NewCertPool()
	pool.AddCert(ca.Cert)

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{node.CertDER},
			PrivateKey:  node.Key,
			Leaf:        node.Cert,
		}},
		RootCAs:    pool,
		MinVersion: tls.VersionTLS13,
	}
}

// --- Persistence ---

const (
	fileCACert  = "ca.pem"
	fileCAKey   = "ca-key.pem"
	fileNodeCert = "node.pem"
	fileNodeKey  = "node-key.pem"
)

// SaveCerts writes the CA and node certificates and keys to dir as PEM files.
func SaveCerts(dir string, ca *CA, node *NodeCert) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("certmanager: mkdir %s: %w", dir, err)
	}

	caKeyBytes, err := x509.MarshalPKCS8PrivateKey(ca.Key)
	if err != nil {
		return fmt.Errorf("certmanager: marshal CA key: %w", err)
	}
	nodeKeyBytes, err := x509.MarshalPKCS8PrivateKey(node.Key)
	if err != nil {
		return fmt.Errorf("certmanager: marshal node key: %w", err)
	}

	files := map[string]*pem.Block{
		fileCACert:   {Type: "CERTIFICATE", Bytes: ca.CertDER},
		fileCAKey:    {Type: "PRIVATE KEY", Bytes: caKeyBytes},
		fileNodeCert: {Type: "CERTIFICATE", Bytes: node.CertDER},
		fileNodeKey:  {Type: "PRIVATE KEY", Bytes: nodeKeyBytes},
	}

	for name, block := range files {
		data := pem.EncodeToMemory(block)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			return fmt.Errorf("certmanager: write %s: %w", name, err)
		}
	}
	return nil
}

// LoadCerts reads CA and node certificates and keys from dir.
// Returns an error if any file is missing or malformed.
func LoadCerts(dir string) (ca *CA, node *NodeCert, err error) {
	caCertDER, err := loadPEM(filepath.Join(dir, fileCACert), "CERTIFICATE")
	if err != nil {
		return nil, nil, err
	}
	caKeyBytes, err := loadPEM(filepath.Join(dir, fileCAKey), "PRIVATE KEY")
	if err != nil {
		return nil, nil, err
	}
	nodeCertDER, err := loadPEM(filepath.Join(dir, fileNodeCert), "CERTIFICATE")
	if err != nil {
		return nil, nil, err
	}
	nodeKeyBytes, err := loadPEM(filepath.Join(dir, fileNodeKey), "PRIVATE KEY")
	if err != nil {
		return nil, nil, err
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: parse CA cert: %w", err)
	}
	caKey, err := parseED25519Key(caKeyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: parse CA key: %w", err)
	}

	nodeCert, err := x509.ParseCertificate(nodeCertDER)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: parse node cert: %w", err)
	}
	nodeKey, err := parseED25519Key(nodeKeyBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("certmanager: parse node key: %w", err)
	}

	return &CA{Cert: caCert, CertDER: caCertDER, Key: caKey},
		&NodeCert{Cert: nodeCert, CertDER: nodeCertDER, Key: nodeKey},
		nil
}

// CertsExist returns true if all cert files exist in dir.
func CertsExist(dir string) bool {
	for _, f := range []string{fileCACert, fileCAKey, fileNodeCert, fileNodeKey} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			return false
		}
	}
	return true
}

// --- helpers ---

func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("certmanager: generate serial: %w", err)
	}
	return serial, nil
}

func loadPEM(path, expectedType string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("certmanager: read %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("certmanager: no PEM block in %s", path)
	}
	if block.Type != expectedType {
		return nil, fmt.Errorf("certmanager: %s: expected %s, got %s", path, expectedType, block.Type)
	}
	return block.Bytes, nil
}

// ParseKey parses a PKCS8-encoded ED25519 private key.
func ParseKey(pkcs8 []byte) (ed25519.PrivateKey, error) {
	return parseED25519Key(pkcs8)
}

func parseED25519Key(pkcs8 []byte) (ed25519.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(pkcs8)
	if err != nil {
		return nil, err
	}
	edKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		// Try crypto.Signer interface (some implementations wrap ed25519)
		if signer, ok := key.(crypto.Signer); ok {
			if pub, ok := signer.Public().(ed25519.PublicKey); ok {
				_ = pub
				return nil, fmt.Errorf("certmanager: key is crypto.Signer with ed25519 public key but not ed25519.PrivateKey")
			}
		}
		return nil, fmt.Errorf("certmanager: expected ed25519 key, got %T", key)
	}
	return edKey, nil
}
