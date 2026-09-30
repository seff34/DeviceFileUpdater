package transport

import (
	"context"
	"errors"
	"testing"

	"devupdater/internal/testutil"
)

func handler(cmd string) (string, int) {
	switch cmd {
	case "uname":
		return "Linux", 0
	case "fail":
		return "boom", 3
	}
	return "", 0
}

func TestSSHExec(t *testing.T) {
	for _, noExec := range []bool{false, true} {
		addr := testutil.FakeSSH{User: "dev", Pass: "pw", NoExec: noExec, Handler: handler}.Start(t)
		s, err := DialSSH(context.Background(), addr, "dev", "pw", testOpts())
		if err != nil {
			t.Fatalf("noExec=%v: %v", noExec, err)
		}
		out, code, err := s.Exec(context.Background(), "uname")
		if err != nil || code != 0 || out != "Linux" {
			t.Fatalf("noExec=%v: out=%q code=%d err=%v", noExec, out, code, err)
		}
		out, code, _ = s.Exec(context.Background(), "fail")
		if code != 3 || out != "boom" {
			t.Fatalf("noExec=%v fail: out=%q code=%d", noExec, out, code)
		}
		if s.Protocol() != "ssh" {
			t.Fatal(s.Protocol())
		}
		s.Close()
	}
}

func TestSSHBadPassword(t *testing.T) {
	addr := testutil.FakeSSH{User: "dev", Pass: "pw", Handler: handler}.Start(t)
	_, err := DialSSH(context.Background(), addr, "dev", "nope", testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestIsSSHAuthError(t *testing.T) {
	for msg, want := range map[string]bool{
		"ssh: handshake failed: ssh: unable to authenticate, attempted methods [none password]": true,
		"ssh: handshake failed: ssh: unexpected message type 51 (expected 60)":                  true,
		"ssh: handshake failed: EOF": false,
	} {
		if got := isSSHAuthError(errors.New(msg)); got != want {
			t.Errorf("%q: got %v want %v", msg, got, want)
		}
	}
}
