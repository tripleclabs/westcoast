package cluster

import (
	"crypto/subtle"
	"errors"
)

var (
	// ErrAuthRejected is returned when a peer's credentials fail verification.
	ErrAuthRejected = errors.New("cluster_auth_rejected")
)

// ClusterAuth handles mutual authentication during transport handshake.
// Authentication happens at connection establishment time, not per-message.
type ClusterAuth interface {
	// Credentials returns the auth payload to send during handshake.
	Credentials() ([]byte, error)
	// Verify checks the peer's credentials. Returns nil on success.
	Verify(peerCredentials []byte) error
}

// NoopAuth accepts all connections without authentication.
type NoopAuth struct{}

func (NoopAuth) Credentials() ([]byte, error) { return nil, nil }
func (NoopAuth) Verify([]byte) error          { return nil }

// SharedSecretAuth authenticates using a pre-shared cluster cookie.
// Uses constant-time comparison to prevent timing attacks.
type SharedSecretAuth struct {
	Secret []byte
}

// NewSharedSecretAuth returns a SharedSecretAuth that authenticates using the given secret.
func NewSharedSecretAuth(secret []byte) *SharedSecretAuth {
	s := make([]byte, len(secret))
	copy(s, secret)
	return &SharedSecretAuth{Secret: s}
}

// Credentials returns a copy of the shared secret as the auth payload.
func (a *SharedSecretAuth) Credentials() ([]byte, error) {
	out := make([]byte, len(a.Secret))
	copy(out, a.Secret)
	return out, nil
}

// Verify checks that the peer's credentials match the shared secret using constant-time comparison.
func (a *SharedSecretAuth) Verify(peerCredentials []byte) error {
	if subtle.ConstantTimeCompare(a.Secret, peerCredentials) != 1 {
		return ErrAuthRejected
	}
	return nil
}
