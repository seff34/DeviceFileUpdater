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
)

func TestProbeParsesTools(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\nHAVE printf\ngarbage\nHAVE od", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
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
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
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
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
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
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("sha256sum") || !c.Has("md5sum") {
		t.Fatalf("caps %+v", c)
	}
}

func TestProbeExecError(t *testing.T) {
	errSession := &errorSession{err: errors.New("connection failed")}
	c, err := Probe(context.Background(), errSession, "127.0.0.1", 200*time.Millisecond)
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
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !c.FTP {
		t.Fatalf("expected FTP=true, got %+v", c)
	}
}

func TestProbeFTPClosedPort(t *testing.T) {
	// Use a port that's unlikely to be open; high port number
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 100*time.Millisecond)
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
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
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

// fakeSSHSession mimics SSHSession with Client() method
type fakeSSHSession struct {
	client *fakeSSHClient
}

func (f *fakeSSHSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	return "", 0, nil
}

func (f *fakeSSHSession) Protocol() string { return "ssh" }

func (f *fakeSSHSession) Close() error { return nil }

func (f *fakeSSHSession) Client() *fakeSSHClient { return f.client }

// fakeSSHClient mimics ssh.Client but isn't one
type fakeSSHClient struct{}
