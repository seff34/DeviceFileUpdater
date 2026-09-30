//go:build !windows

package shell

import (
	"os"
	"os/exec"
	"testing"
)

// shSession runs a MarkerSession against a real /bin/sh reading commands from stdin.
func shSession(t *testing.T) *MarkerSession {
	t.Helper()
	cmd := exec.Command("/bin/sh")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = w, w
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	w.Close()
	m := NewMarkerSession(r, stdin, "\n")
	t.Cleanup(func() { stdin.Close(); cmd.Wait(); m.Close(); r.Close() })
	return m
}

// The marker must not be glued onto the command line: a trailing `&`, `;` or
// comment would otherwise break (or swallow) the marker echo.
func TestExecCommandShapesRealShell(t *testing.T) {
	m := shSession(t)
	for _, tc := range []struct {
		cmd, out string
		code     int
	}{
		{"sleep 0 &", "", 0},
		{"true;", "", 0},
		{"true # c", "", 0},
		{"false;", "", 1},
		{"(exit 3) # c", "", 3},
		{"echo hi # c", "hi", 0},
	} {
		out, code, err := m.Exec(ctx(t), tc.cmd)
		if err != nil || code != tc.code || out != tc.out {
			t.Fatalf("%q: out=%q code=%d err=%v", tc.cmd, out, code, err)
		}
	}
}
