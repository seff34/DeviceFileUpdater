package shell

import (
	"context"
	"errors"
	"io"
	"regexp"
	"runtime"
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

// byteReader delivers one byte per Read, like a worst-case TCP split.
type byteReader struct{ r io.Reader }

func (b byteReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	return b.r.Read(p[:1])
}

func TestExecSplitNewlineAfterMarker(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	go testutil.ServeFakeShell(sr, sw, false, func(cmd string) (string, int) {
		if cmd == "a" {
			return "A", 0
		}
		return "x", 0
	})
	t.Cleanup(func() { cw.Close(); sw.Close() })
	m := NewMarkerSession(byteReader{cr}, cw, "\n")
	for _, tc := range [][2]string{{"a", "A"}, {"printf x", "x"}, {"b", "x"}} {
		out, code, err := m.Exec(ctx(t), tc[0])
		if err != nil || code != 0 || out != tc[1] {
			t.Fatalf("%s: out=%q code=%d err=%v", tc[0], out, code, err)
		}
	}
}

func TestExecStreamClosedMidCommand(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	go testutil.ServeFakeShell(sr, sw, false, func(string) (string, int) {
		sw.Close()
		return "", 0
	})
	t.Cleanup(func() { cw.Close() })
	m := NewMarkerSession(cr, cw, "\n")
	if _, _, err := m.Exec(ctx(t), "x"); !errors.Is(err, io.EOF) {
		t.Fatalf("want EOF, got %v", err)
	}
	if _, _, err := m.Exec(ctx(t), "y"); err == nil {
		t.Fatal("expected error after closed stream")
	}
}

func pumpCount() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "shell.(*MarkerSession).pump")
}

// endless never stops producing data, so the pump fills its channel and blocks.
type endless struct{}

func (endless) Read(p []byte) (int, error) { p[0] = 'x'; return 1, nil }

func TestCloseStopsReader(t *testing.T) {
	before := pumpCount()
	m := NewMarkerSession(endless{}, io.Discard, "\n")
	time.Sleep(20 * time.Millisecond) // let the pump fill its buffer and block
	m.Close()
	deadline := time.Now().Add(2 * time.Second)
	for pumpCount() > before {
		if time.Now().After(deadline) {
			t.Fatal("reader goroutine still running after Close")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, _, err := m.Exec(ctx(t), "x"); err == nil {
		t.Fatal("Exec after Close must fail")
	}
}
