package transport

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

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

// returnsWithin fails the test if fn does not return within d.
func returnsWithin(t *testing.T, d time.Duration, fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-time.After(d):
		t.Fatalf("call did not return within %v", d)
		return nil
	}
}

func TestSSHExecHonoursCtxWhenChannelOpenStalls(t *testing.T) {
	hold := &atomic.Bool{}
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: handler, HoldChannels: hold}.Start(t)
	s, err := DialSSH(context.Background(), addr, "u", "p", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	hold.Store(true)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = returnsWithin(t, 2*time.Second, func() error { _, _, err := s.Exec(ctx, "uname"); return err })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}

func TestSSHExecHonoursCtxWhenStartStalls(t *testing.T) {
	stall := func(kind, arg string) bool { return kind == "exec" && arg == "hang" }
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: handler, Stall: stall}.Start(t)
	s, err := DialSSH(context.Background(), addr, "u", "p", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = returnsWithin(t, 2*time.Second, func() error { _, _, err := s.Exec(ctx, "hang"); return err })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
}
