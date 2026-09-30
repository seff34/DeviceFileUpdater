//go:build !windows

package upload

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devupdater/internal/testutil"
	"devupdater/internal/transport"

	"golang.org/x/crypto/ssh"
)

func dialFake(t *testing.T) transport.Session {
	t.Helper()
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: func(string) (string, int) { return "", 0 }}.Start(t)
	s, err := transport.DialSSH(context.Background(), addr, "u", "p",
		transport.Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSFTPUpload(t *testing.T) {
	s := dialFake(t)
	dst := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dst, []byte("old content that is longer"), 0o644)
	u := NewSFTP(s.(interface{ Client() *ssh.Client }).Client())
	if err := u.Upload(context.Background(), []byte("new"), dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new" {
		t.Fatalf("got %q (must truncate)", got)
	}
}

func TestSFTPUploadCancelled(t *testing.T) {
	s := dialFake(t)
	dst := filepath.Join(t.TempDir(), "f.bin")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	u := NewSFTP(s.(interface{ Client() *ssh.Client }).Client())
	if err := u.Upload(ctx, []byte("x"), dst); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("cancelled upload must not create the file")
	}
}
