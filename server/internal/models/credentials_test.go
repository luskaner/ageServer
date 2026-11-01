package models

import (
	"testing"

	"github.com/luskaner/ageServer/server/internal"
)

func TestGenerateSignature(t *testing.T) {
	// Need to initialize rng first
	internal.InitializeRng(42)
	sig1 := generateSignature()
	sig2 := generateSignature()
	if sig1 == sig2 {
		t.Fatal("signatures should be different")
	}
	if len(sig1) == 0 {
		t.Fatal("empty signature")
	}
}

func TestNewCredentialsAndCreate(t *testing.T) {
	creds := NewCredentials()
	if creds == nil {
		t.Fatal("nil creds")
	}
	key := "testkey"
	cred := CreateCredential(creds, &key)
	if cred == nil {
		t.Fatal("nil cred")
	}
	// Test with nil key
	cred2 := CreateCredential(creds, nil)
	if cred2 == nil {
		t.Fatal("nil cred2")
	}
}
