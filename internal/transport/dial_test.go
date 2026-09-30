package transport

import (
	"context"
	"errors"
	"strings"
	"testing"

	"devupdater/internal/testutil"
	"devupdater/internal/workspace"
)

func TestDialPrefersSSH(t *testing.T) {
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: handler}.Start(t)
	s, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "p"}, testOpts())
	if err != nil || s.Protocol() != "ssh" {
		t.Fatalf("s=%v err=%v", s, err)
	}
	s.Close()
}

func TestDialFallsBackToTelnet(t *testing.T) {
	// Same port for both attempts: the SSH handshake fails against a telnet server.
	addr := testutil.StartFakeTelnet(t, "u", "p", handler)
	s, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "p"}, testOpts())
	if err != nil || s.Protocol() != "telnet" {
		t.Fatalf("s=%v err=%v", s, err)
	}
	s.Close()
}

func TestDialBothFail(t *testing.T) {
	_, err := Dial(context.Background(), workspace.Device{Host: "127.0.0.1:1", Username: "u", Password: "p"}, testOpts())
	if err == nil || !strings.Contains(err.Error(), "ssh") || !strings.Contains(err.Error(), "telnet") {
		t.Fatalf("expected combined error, got %v", err)
	}
}

func TestDialAuthNotRetried(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "u", "p", handler)
	_, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "bad"}, testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}
