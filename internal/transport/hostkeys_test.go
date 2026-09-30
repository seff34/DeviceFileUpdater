package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newKey(t *testing.T) ssh.PublicKey {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestTOFU(t *testing.T) {
	p := filepath.Join(t.TempDir(), "known_hosts")
	k1, k2 := newKey(t), newKey(t)
	if err := checkKnownHost(p, "10.0.0.1:22", k1); err != nil {
		t.Fatal(err)
	}
	if err := checkKnownHost(p, "10.0.0.1:22", k1); err != nil {
		t.Fatal("same key must pass:", err)
	}
	if err := checkKnownHost(p, "10.0.0.1:22", k2); err == nil {
		t.Fatal("changed key must fail")
	}
	if err := checkKnownHost(p, "10.0.0.2:22", k2); err != nil {
		t.Fatal("other host must pass:", err)
	}
}
