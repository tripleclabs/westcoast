package cluster

import (
	"crypto/x509"
	"errors"
	"fmt"
)

// CertAuth authenticates peers using TLS certificates. It verifies that
// the peer's certificate was signed by the expected cluster CA.
//
// When used with a TLS-enabled gRPC transport, the TLS handshake handles
// the actual certificate exchange. CertAuth provides the ClusterAuth
// handshake credentials: the node sends its certificate's raw bytes as
// credentials, and the peer verifies the certificate against the CA.
//
// This allows cryptographic node identity — the node ID can be extracted
// from the certificate's Common Name rather than trusting the HELO payload.
type CertAuth struct {
	// CACert is the cluster CA certificate used to verify peer certificates.
	CACert *x509.Certificate

	// NodeCert is this node's certificate, sent as credentials during handshake.
	NodeCert *x509.Certificate
}

// NewCertAuth creates a CertAuth that verifies peers against the given CA.
func NewCertAuth(caCert, nodeCert *x509.Certificate) *CertAuth {
	return &CertAuth{CACert: caCert, NodeCert: nodeCert}
}

// Credentials returns this node's certificate as DER-encoded bytes.
func (a *CertAuth) Credentials() ([]byte, error) {
	if a.NodeCert == nil {
		return nil, errors.New("cert_auth: no node certificate configured")
	}
	return a.NodeCert.Raw, nil
}

// Verify checks that the peer's certificate was signed by the cluster CA.
func (a *CertAuth) Verify(peerCredentials []byte) error {
	if a.CACert == nil {
		return errors.New("cert_auth: no CA certificate configured")
	}
	if len(peerCredentials) == 0 {
		return fmt.Errorf("%w: no certificate provided", ErrAuthRejected)
	}

	peerCert, err := x509.ParseCertificate(peerCredentials)
	if err != nil {
		return fmt.Errorf("%w: invalid certificate: %v", ErrAuthRejected, err)
	}

	// Verify the peer cert was signed by our cluster CA.
	roots := x509.NewCertPool()
	roots.AddCert(a.CACert)

	_, err = peerCert.Verify(x509.VerifyOptions{
		Roots: roots,
	})
	if err != nil {
		return fmt.Errorf("%w: certificate verification failed: %v", ErrAuthRejected, err)
	}

	return nil
}

// PeerNodeID extracts the node ID from a peer's certificate Common Name.
// Call this after Verify to get the authenticated node identity.
func PeerNodeID(peerCredentials []byte) (NodeID, error) {
	cert, err := x509.ParseCertificate(peerCredentials)
	if err != nil {
		return "", fmt.Errorf("parse peer cert: %w", err)
	}
	if cert.Subject.CommonName == "" {
		return "", errors.New("peer certificate has no Common Name")
	}
	return NodeID(cert.Subject.CommonName), nil
}
