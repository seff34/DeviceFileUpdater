package upload

import (
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"

	"golang.org/x/crypto/ssh"
)

func names(us []Uploader) []string {
	var out []string
	for _, u := range us {
		out = append(out, u.Name())
	}
	return out
}

func checkNames(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestSelectOrderNonSSH(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	c := probe.Caps{Tools: map[string]bool{"base64": true, "scp": true}, FTP: true, SFTP: true}
	// no ssh client → no sftp/scp
	checkNames(t, names(Select(s, c, FTPCreds{}, time.Second)), []string{"ftp", "shell-base64", "shell-printf"})
}

func TestSelectMinimal(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	checkNames(t, names(Select(s, probe.Caps{Tools: map[string]bool{}}, FTPCreds{}, time.Second)), []string{"shell-printf"})
}

// nilClientSession has a Client() method that returns nil.
type nilClientSession struct{ testutil.FakeSession }

func (n *nilClientSession) Client() *ssh.Client { return nil }

func TestSelectNilSSHClient(t *testing.T) {
	s := &nilClientSession{}
	c := probe.Caps{Tools: map[string]bool{"scp": true}, SFTP: true}
	checkNames(t, names(Select(s, c, FTPCreds{}, time.Second)), []string{"shell-printf"})
}
