package transport

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devupdater/internal/testutil"

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

func TestKnownHostsCRLF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "known_hosts")
	k := newKey(t)
	// Hand-edited on Windows: CRLF plus stray trailing whitespace.
	line := "10.0.0.1:22 " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k))) + " \t\r\n"
	if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkKnownHost(p, "10.0.0.1:22", k); err != nil {
		t.Fatal("CRLF line with the same key must pass:", err)
	}
	if b, _ := os.ReadFile(p); string(b) != line {
		t.Fatalf("known_hosts must not gain a duplicate line: %q", b)
	}
}

func TestSSHHostKeyMismatch(t *testing.T) {
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: func(string) (string, int) { return "", 0 }}.Start(t)
	p := filepath.Join(t.TempDir(), "known_hosts")
	line := addr + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(newKey(t)))) + "\n"
	if err := os.WriteFile(p, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	opt := testOpts()
	opt.StrictHostKey, opt.KnownHostsPath = true, p
	_, err := DialSSH(context.Background(), addr, "u", "p", opt)
	if !errors.Is(err, ErrHostKey) {
		t.Fatalf("err = %v, want ErrHostKey", err)
	}
}
