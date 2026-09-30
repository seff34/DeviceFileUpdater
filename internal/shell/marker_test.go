package shell

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

// pipeShell connects a MarkerSession to a fake shell and returns it.
func pipeShell(t *testing.T, echo bool, h func(string) (string, int)) *MarkerSession {
	t.Helper()
	cr, sw := io.Pipe() // shell -> client
	sr, cw := io.Pipe() // client -> shell
	go testutil.ServeFakeShell(sr, sw, echo, h)
	t.Cleanup(func() { cw.Close(); sw.Close() })
	return NewMarkerSession(cr, cw, "\n")
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestExecOutputAndCode(t *testing.T) {
	for _, echo := range []bool{false, true} {
		m := pipeShell(t, echo, func(cmd string) (string, int) {
			if cmd == "false" {
				return "", 1
			}
			return "line1\nline2", 0
		})
		out, code, err := m.Exec(ctx(t), "cat x")
		if err != nil || code != 0 || out != "line1\nline2" {
			t.Fatalf("echo=%v: out=%q code=%d err=%v", echo, out, code, err)
		}
		out, code, err = m.Exec(ctx(t), "false")
		if err != nil || code != 1 || out != "" {
			t.Fatalf("echo=%v second: out=%q code=%d err=%v", echo, out, code, err)
		}
	}
}

func TestExecTimeoutBreaksSession(t *testing.T) {
	block := make(chan struct{})
	m := pipeShell(t, false, func(string) (string, int) { <-block; return "", 0 })
	defer close(block)
	c, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := m.Exec(c, "sleep"); err == nil {
		t.Fatal("expected timeout")
	}
	if _, _, err := m.Exec(ctx(t), "echo"); err == nil || !strings.Contains(err.Error(), "out of sync") {
		t.Fatalf("expected out of sync, got %v", err)
	}
}

func TestExpect(t *testing.T) {
	cr, sw := io.Pipe()
	m := NewMarkerSession(cr, io.Discard, "\n")
	go io.WriteString(sw, "Welcome\r\nbox login: ")
	got, err := m.Expect(ctx(t), regexp.MustCompile(`login: $`))
	if err != nil || !strings.HasSuffix(got, "login: ") {
		t.Fatalf("got %q err %v", got, err)
	}
}
