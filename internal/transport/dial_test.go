package transport

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestConnectionErrorRetried(t *testing.T) {
	// Mock dial functions that return connection errors.
	// Verify each is called exactly 2 times (once per attempt).
	var sshCalls, telnetCalls int
	oldDialSSH := dialSSH
	oldDialTelnet := dialTelnet
	t.Cleanup(func() {
		dialSSH = oldDialSSH
		dialTelnet = oldDialTelnet
	})

	dialSSH = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		sshCalls++
		return nil, errors.New("connection refused")
	}
	dialTelnet = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		telnetCalls++
		return nil, errors.New("connection timeout")
	}

	_, err := Dial(context.Background(), workspace.Device{Host: "127.0.0.1:999", Username: "u", Password: "p"}, testOpts())
	if err == nil {
		t.Fatal("expected error")
	}
	if sshCalls != 2 {
		t.Fatalf("expected SSH called 2 times, got %d", sshCalls)
	}
	if telnetCalls != 2 {
		t.Fatalf("expected Telnet called 2 times, got %d", telnetCalls)
	}
}

func TestErrAuthOnceNotRetried(t *testing.T) {
	// When SSH returns ErrAuth, Dial returns immediately without retrying.
	var sshCalls, telnetCalls int
	oldDialSSH := dialSSH
	oldDialTelnet := dialTelnet
	t.Cleanup(func() {
		dialSSH = oldDialSSH
		dialTelnet = oldDialTelnet
	})

	dialSSH = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		sshCalls++
		return nil, ErrAuth
	}
	dialTelnet = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		telnetCalls++
		return nil, errors.New("should not retry telnet after ErrAuth")
	}

	_, err := Dial(context.Background(), workspace.Device{Host: "127.0.0.1:999", Username: "u", Password: "p"}, testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
	if sshCalls != 1 {
		t.Fatalf("expected SSH called 1 time, got %d", sshCalls)
	}
	if telnetCalls != 1 {
		t.Fatalf("expected Telnet called 1 time, got %d", telnetCalls)
	}
}

func TestDialContextCancelledDuringRetry(t *testing.T) {
	// Mock dial functions and cancel context during retry wait.
	// Verify Dial returns context.Canceled error quickly.
	var sshCalls int
	oldDialSSH := dialSSH
	oldDialTelnet := dialTelnet
	oldRetryDelay := retryDelay
	t.Cleanup(func() {
		dialSSH = oldDialSSH
		dialTelnet = oldDialTelnet
		retryDelay = oldRetryDelay
	})

	dialSSH = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		sshCalls++
		return nil, errors.New("connection error")
	}

	// Mock dialTelnet that should not be called on second attempt due to context cancellation
	dialTelnet = func(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
		return nil, errors.New("should not reach telnet on second attempt")
	}

	retryDelay = 10 * time.Second // Long delay to ensure we can cancel during wait

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after first attempt but before retry wait completes
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Dial(ctx, workspace.Device{Host: "127.0.0.1:999", Username: "u", Password: "p"}, testOpts())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if sshCalls != 1 {
		t.Fatalf("expected SSH called 1 time, got %d", sshCalls)
	}
}

// A host key mismatch must never fall back to Telnet (cleartext password) or retry.
func TestDialHostKeyMismatchStops(t *testing.T) {
	var sshCalls, telnetCalls int
	oldDialSSH, oldDialTelnet := dialSSH, dialTelnet
	t.Cleanup(func() { dialSSH, dialTelnet = oldDialSSH, oldDialTelnet })
	dialSSH = func(context.Context, string, string, string, Options) (Session, error) {
		sshCalls++
		return nil, fmt.Errorf("ssh handshake x: %w", ErrHostKey)
	}
	dialTelnet = func(context.Context, string, string, string, Options) (Session, error) {
		telnetCalls++
		return nil, errors.New("telnet must not be tried")
	}
	_, err := Dial(context.Background(), workspace.Device{Host: "127.0.0.1:999", Username: "u", Password: "p"}, testOpts())
	if !errors.Is(err, ErrHostKey) || !strings.Contains(err.Error(), "host key mismatch") {
		t.Fatalf("err = %v, want ErrHostKey", err)
	}
	if sshCalls != 1 || telnetCalls != 0 {
		t.Fatalf("ssh calls %d (want 1), telnet calls %d (want 0)", sshCalls, telnetCalls)
	}
}
