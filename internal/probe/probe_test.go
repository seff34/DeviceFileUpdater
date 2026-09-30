package probe

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"

	"golang.org/x/crypto/ssh"
)

func TestProbeParsesTools(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\nHAVE printf\ngarbage\nHAVE od", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("md5sum") || !c.Has("printf") || !c.Has("od") || c.Has("base64") || c.SFTP {
		t.Fatalf("caps %+v", c)
	}
	if !strings.Contains(s.Cmds[0], "command -v") {
		t.Fatalf("probe cmd %q", s.Cmds[0])
	}
}

func TestProbeParsesCRLF(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\r\nHAVE printf\r\n", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("md5sum") || !c.Has("printf") {
		t.Fatalf("caps %+v", c)
	}
}

func TestProbeIgnoresInvalidToolNames(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\nHAVE fakeTool\nHAVE printf", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("md5sum") || !c.Has("printf") || c.Has("fakeTool") {
		t.Fatalf("caps %+v", c)
	}
}

func TestProbeParsePromptNoise(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "~ # HAVE sha256sum\nroot@device: HAVE md5sum", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("sha256sum") || !c.Has("md5sum") {
		t.Fatalf("caps %+v", c)
	}
}

func TestProbeExecError(t *testing.T) {
	errSession := &errorSession{err: errors.New("connection failed")}
	c, err := Probe(context.Background(), errSession, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "connection failed" {
		t.Fatalf("wrong error: %v", err)
	}
	if len(c.Tools) != 0 {
		t.Fatalf("caps should be empty: %+v", c)
	}
}

func TestProbeFTPWithListener(t *testing.T) {
	// Create a local listener on a random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	port := fmt.Sprintf("%d", addr.Port)

	// Save original ftpPort and restore it
	originalPort := ftpPort
	ftpPort = port
	t.Cleanup(func() { ftpPort = originalPort })

	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !c.FTP {
		t.Fatalf("expected FTP=true, got %+v", c)
	}
}

func TestProbeFTPClosedPort(t *testing.T) {
	// Create a listener, get the port, close it so it's unavailable
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create listener: %v", err)
	}
	addr := listener.Addr().(*net.TCPAddr)
	port := fmt.Sprintf("%d", addr.Port)
	listener.Close() // Close it so the port is no longer listening

	// Save and restore ftpPort
	originalPort := ftpPort
	ftpPort = port
	t.Cleanup(func() { ftpPort = originalPort })

	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.FTP {
		t.Fatalf("expected FTP=false for closed port, got %+v", c)
	}
}

func TestProbeListSorted(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE stat\nHAVE base64\nHAVE sha256sum", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	list := c.List()
	// Should be sorted
	for i := 1; i < len(list); i++ {
		if list[i] < list[i-1] {
			t.Fatalf("list not sorted: %v", list)
		}
	}
	// Check all are present
	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d: %v", len(list), list)
	}
}

func TestProbeListIncludesSFTP(t *testing.T) {
	c := Caps{
		Tools: map[string]bool{"md5sum": true},
		SFTP:  true,
		FTP:   false,
	}
	list := c.List()
	// Should include sftp
	found := false
	for _, name := range list {
		if name == "sftp" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'sftp' in list: %v", list)
	}
}

func TestProbeListIncludesFTP(t *testing.T) {
	c := Caps{
		Tools: map[string]bool{"md5sum": true},
		SFTP:  false,
		FTP:   true,
	}
	list := c.List()
	// Should include ftp
	found := false
	for _, name := range list {
		if name == "ftp" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'ftp' in list: %v", list)
	}
}

// errorSession is a fake that returns errors
type errorSession struct {
	err error
}

func (e *errorSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	return "", 0, e.err
}

func (e *errorSession) Protocol() string { return "error" }

func (e *errorSession) Close() error { return nil }

// nilClientSession has a Client() method that returns nil
type nilClientSession struct{}

func (n *nilClientSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	return "HAVE md5sum", 0, nil
}

func (n *nilClientSession) Protocol() string { return "ssh" }

func (n *nilClientSession) Close() error { return nil }

func (n *nilClientSession) Client() *ssh.Client { return nil }

func TestProbeNilClient(t *testing.T) {
	// Test that nil Client() doesn't panic and SFTP=false
	s := &nilClientSession{}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.SFTP {
		t.Fatalf("expected SFTP=false for nil client, got %+v", c)
	}
	if !c.Has("md5sum") {
		t.Fatalf("should have parsed tools, got %+v", c)
	}
}

// stallSession blocks every command until its ctx ends, like a wedged device.
type stallSession struct{}

func (stallSession) Exec(ctx context.Context, _ string) (string, int, error) {
	<-ctx.Done()
	return "", -1, ctx.Err()
}
func (stallSession) Protocol() string { return "telnet" }
func (stallSession) Close() error     { return nil }

func TestProbeCommandTimeout(t *testing.T) {
	done := make(chan error, 1)
	go func() {
		_, err := Probe(context.Background(), stallSession{}, "127.0.0.1", 200*time.Millisecond, 100*time.Millisecond)
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want deadline exceeded", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("probe hung on a stalled command")
	}
}

// An empty ftpHost (device given with an explicit port) skips FTP detection.
func TestProbeSkipsFTPWithoutHost(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	old := ftpPort
	ftpPort = fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	t.Cleanup(func() { ftpPort = old })
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	c, err := Probe(context.Background(), s, "", 200*time.Millisecond, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if c.FTP {
		t.Fatalf("FTP must not be probed without a host: %+v", c)
	}
}
