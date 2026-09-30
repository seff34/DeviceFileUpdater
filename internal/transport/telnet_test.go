package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func testOpts() Options {
	return Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second}
}

func TestTelnetExec(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(cmd string) (string, int) {
		if cmd == "uname" {
			return "Linux", 0
		}
		return "", 0
	})
	s, err := DialTelnet(context.Background(), addr, "dev", "pw", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Protocol() != "telnet" {
		t.Fatalf("protocol %s", s.Protocol())
	}
	out, code, err := s.Exec(context.Background(), "uname")
	if err != nil || code != 0 || out != "Linux" {
		t.Fatalf("out=%q code=%d err=%v", out, code, err)
	}
}

func TestTelnetBadPassword(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(string) (string, int) { return "", 0 })
	_, err := DialTelnet(context.Background(), addr, "dev", "wrong", testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestTelnetRefused(t *testing.T) {
	_, err := DialTelnet(context.Background(), "127.0.0.1:1", "a", "b", testOpts())
	if err == nil || errors.Is(err, ErrAuth) {
		t.Fatalf("expected connection error, got %v", err)
	}
}
