//go:build !windows

package upload

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func allBytes() []byte {
	var b bytes.Buffer
	for i := 0; i < 12; i++ {
		for c := 0; c < 256; c++ {
			b.WriteByte(byte(c))
		}
	}
	b.WriteString("-%s'\\ end")
	return b.Bytes()
}

func TestShellUploadersRoundTrip(t *testing.T) {
	for _, u := range []Uploader{
		NewShellBase64(testutil.LocalShell{}, 5*time.Second),
		NewShellPrintf(testutil.LocalShell{}, 5*time.Second),
	} {
		for _, data := range [][]byte{allBytes(), {}, []byte("-leading dash"), []byte(" 5-1%0 9\x0012\n7")} {
			dst := filepath.Join(t.TempDir(), "sub dir", "out.bin")
			os.MkdirAll(filepath.Dir(dst), 0o755)
			// pre-existing longer content must be truncated
			os.WriteFile(dst, bytes.Repeat([]byte("Z"), 50000), 0o644)
			if err := u.Upload(context.Background(), data, dst); err != nil {
				t.Fatalf("%s: %v", u.Name(), err)
			}
			got, _ := os.ReadFile(dst)
			if !bytes.Equal(got, data) {
				t.Fatalf("%s: content mismatch (len %d vs %d)", u.Name(), len(got), len(data))
			}
			if _, err := os.Stat(dst + ".b64"); err == nil {
				t.Fatalf("%s: leftover .b64", u.Name())
			}
		}
	}
}

func TestPrintfCommandsStayShort(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	if err := NewShellPrintf(s, time.Second).Upload(context.Background(), allBytes(), "/x"); err != nil {
		t.Fatal(err)
	}
	if len(s.Cmds) < 2 {
		t.Fatalf("expected chunking, got %d commands", len(s.Cmds))
	}
	for _, c := range s.Cmds {
		if len(c) > 1024 {
			t.Fatalf("command too long: %d", len(c))
		}
	}
	if !strings.HasSuffix(s.Cmds[0], " > '/x'") || !strings.HasSuffix(s.Cmds[1], " >> '/x'") {
		t.Fatalf("first chunk must truncate, later append: %q / %q", s.Cmds[0][len(s.Cmds[0])-10:], s.Cmds[1][len(s.Cmds[1])-10:])
	}
}

func TestBase64CommandsStayShort(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	if err := NewShellBase64(s, time.Second).Upload(context.Background(), allBytes(), "/x"); err != nil {
		t.Fatal(err)
	}
	if len(s.Cmds) < 3 {
		t.Fatalf("expected chunking, got %d commands", len(s.Cmds))
	}
	for _, c := range s.Cmds {
		if len(c) > 1024 {
			t.Fatalf("command too long: %d", len(c))
		}
	}
}

func TestChunkFailureAbortsAndCleansUp(t *testing.T) {
	n := 0
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		n++
		if n == 3 {
			return "disk full", 1
		}
		return "", 0
	}}
	err := NewShellBase64(s, time.Second).Upload(context.Background(), allBytes(), "/x")
	if err == nil || !strings.Contains(err.Error(), "disk full") {
		t.Fatalf("expected chunk error, got %v", err)
	}
	if len(s.Cmds) != 4 || !strings.HasPrefix(s.Cmds[3], "rm -f ") {
		t.Fatalf("expected abort then cleanup, cmds=%d", len(s.Cmds))
	}
}

func TestZeroTimeoutIsNotInstant(t *testing.T) {
	for _, u := range []Uploader{
		NewShellBase64(testutil.LocalShell{}, 0),
		NewShellPrintf(testutil.LocalShell{}, -1),
	} {
		dst := filepath.Join(t.TempDir(), "o")
		if err := u.Upload(context.Background(), []byte("hi"), dst); err != nil {
			t.Fatalf("%s: %v", u.Name(), err)
		}
	}
}

func TestUploadHonoursCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := NewShellPrintf(testutil.LocalShell{}, time.Second).Upload(ctx, []byte("x"), filepath.Join(t.TempDir(), "o"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func longPath() string {
	return "/" + strings.Repeat("d", 120) + "/" + strings.Repeat("f", 120) + ".bin"
}

func TestLongPathCommandsStayShort(t *testing.T) {
	remote := longPath()
	for _, u := range []func(*testutil.FakeSession) Uploader{
		func(s *testutil.FakeSession) Uploader { return NewShellBase64(s, time.Second) },
		func(s *testutil.FakeSession) Uploader { return NewShellPrintf(s, time.Second) },
	} {
		s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
		up := u(s)
		if err := up.Upload(context.Background(), allBytes(), remote); err != nil {
			t.Fatalf("%s: %v", up.Name(), err)
		}
		if len(s.Cmds) < 3 {
			t.Fatalf("%s: expected chunking", up.Name())
		}
		for _, c := range s.Cmds {
			if len(c) > 1024-marginLen {
				t.Fatalf("%s: command too long: %d", up.Name(), len(c))
			}
		}
	}
}

func TestLongPathRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("d", 120))
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, strings.Repeat("f", 100)+".bin")
	for _, u := range []Uploader{NewShellBase64(testutil.LocalShell{}, 5*time.Second), NewShellPrintf(testutil.LocalShell{}, 5*time.Second)} {
		if err := u.Upload(context.Background(), allBytes(), dst); err != nil {
			t.Fatalf("%s: %v", u.Name(), err)
		}
		got, _ := os.ReadFile(dst)
		if !bytes.Equal(got, allBytes()) {
			t.Fatalf("%s: mismatch", u.Name())
		}
	}
}

func TestPathTooLongRejectedBeforeSending(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	remote := "/" + strings.Repeat("x", 1000)
	for _, u := range []Uploader{NewShellBase64(s, time.Second), NewShellPrintf(s, time.Second)} {
		if err := u.Upload(context.Background(), []byte("a"), remote); err == nil || !strings.Contains(err.Error(), "too long") {
			t.Fatalf("%s: got %v", u.Name(), err)
		}
	}
	if len(s.Cmds) != 0 {
		t.Fatalf("commands sent: %d", len(s.Cmds))
	}
}

// errSession fails Exec at transport level for commands containing failOn.
type errSession struct {
	testutil.FakeSession
	failOn string
}

func (e *errSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	out, code, _ := e.FakeSession.Exec(ctx, cmd)
	if e.failOn != "" && strings.Contains(cmd, e.failOn) {
		return "", -1, errors.New("link down")
	}
	return out, code, nil
}

func TestBase64CleansUpOnTransportError(t *testing.T) {
	for _, failOn := range []string{": > ", "printf '%s'", "base64 -d"} {
		s := &errSession{FakeSession: testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}, failOn: failOn}
		err := NewShellBase64(s, time.Second).Upload(context.Background(), []byte("hello"), "/x")
		if err == nil || !strings.Contains(err.Error(), "link down") {
			t.Fatalf("%s: got %v", failOn, err)
		}
		if last := s.Cmds[len(s.Cmds)-1]; last != "rm -f '/x.b64'" {
			t.Fatalf("%s: last cmd %q", failOn, last)
		}
	}
}
