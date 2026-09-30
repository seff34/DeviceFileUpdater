//go:build !windows

package upload

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"devupdater/internal/testutil"
	"devupdater/internal/transport"

	"golang.org/x/crypto/ssh"
)

func sshClient(t *testing.T, f testutil.FakeSSH) *ssh.Client {
	t.Helper()
	f.User, f.Pass = "u", "p"
	f.Handler = func(string) (string, int) { return "", 0 }
	s, err := transport.DialSSH(context.Background(), f.Start(t), "u", "p",
		transport.Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s.(interface{ Client() *ssh.Client }).Client()
}

// uploadReturns runs u.Upload with a 100ms deadline and requires it to give up promptly.
func uploadReturns(t *testing.T, u Uploader) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- u.Upload(ctx, []byte("data"), "/tmp/x") }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%s: err = %v, want deadline exceeded", u.Name(), err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: upload hung on a stalled device", u.Name())
	}
}

func TestSFTPStalledSubsystem(t *testing.T) {
	c := sshClient(t, testutil.FakeSSH{Stall: func(kind, _ string) bool { return kind == "subsystem" }})
	uploadReturns(t, NewSFTP(c))
}

func TestSCPStalledStart(t *testing.T) {
	c := sshClient(t, testutil.FakeSSH{Stall: func(kind, arg string) bool { return strings.HasPrefix(arg, "scp ") }})
	uploadReturns(t, NewSCP(c))
}

func TestSCPStalledChannelOpen(t *testing.T) {
	hold := &atomic.Bool{}
	c := sshClient(t, testutil.FakeSSH{HoldChannels: hold})
	hold.Store(true)
	uploadReturns(t, NewSCP(c))
}

func TestFTPStalledServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() { // accept and never send the FTP greeting
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	old := ftpPort
	_, ftpPort, _ = net.SplitHostPort(ln.Addr().String())
	t.Cleanup(func() { ftpPort = old })
	uploadReturns(t, NewFTP(&testutil.FakeSession{}, FTPCreds{Host: "127.0.0.1", User: "u", Pass: "p"}, time.Minute))
}
