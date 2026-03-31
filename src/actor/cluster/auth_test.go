package cluster

import (
	"errors"
	"testing"
)

func TestNoopAuth_AlwaysAccepts(t *testing.T) {
	auth := NoopAuth{}

	creds, err := auth.Credentials()
	if err != nil {
		t.Fatalf("credentials: %v", err)
	}

	if err := auth.Verify(creds); err != nil {
		t.Fatalf("verify: %v", err)
	}

	if err := auth.Verify([]byte("anything")); err != nil {
		t.Fatalf("verify arbitrary: %v", err)
	}
}

func TestSharedSecretAuth_CorrectSecret(t *testing.T) {
	secret := []byte("my-cluster-cookie")
	auth := NewSharedSecretAuth(secret)

	creds, err := auth.Credentials()
	if err != nil {
		t.Fatalf("credentials: %v", err)
	}

	if err := auth.Verify(creds); err != nil {
		t.Fatalf("verify should pass: %v", err)
	}
}

func TestSharedSecretAuth_WrongSecret(t *testing.T) {
	auth := NewSharedSecretAuth([]byte("correct-cookie"))

	if err := auth.Verify([]byte("wrong-cookie")); err == nil {
		t.Fatal("verify should reject wrong secret")
	} else if !errors.Is(err, ErrAuthRejected) {
		t.Fatalf("expected ErrAuthRejected, got %v", err)
	}
}

func TestSharedSecretAuth_EmptySecret(t *testing.T) {
	auth := NewSharedSecretAuth([]byte("secret"))

	if err := auth.Verify([]byte{}); err == nil {
		t.Fatal("verify should reject empty credentials")
	}

	if err := auth.Verify(nil); err == nil {
		t.Fatal("verify should reject nil credentials")
	}
}

func TestSharedSecretAuth_DefensiveCopy(t *testing.T) {
	secret := []byte("original")
	auth := NewSharedSecretAuth(secret)

	// Mutate the original — should not affect the auth.
	secret[0] = 'X'

	creds, _ := auth.Credentials()
	if string(creds) != "original" {
		t.Fatal("auth should hold a defensive copy")
	}

	// Mutate the returned credentials — should not affect future calls.
	creds[0] = 'Y'
	creds2, _ := auth.Credentials()
	if string(creds2) != "original" {
		t.Fatal("credentials should return a defensive copy")
	}
}
