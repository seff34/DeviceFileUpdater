//go:build !windows

package testutil

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// LocalShell runs commands with the machine's /bin/sh — a stand-in device for tests.
type LocalShell struct{}

func (LocalShell) Exec(ctx context.Context, cmd string) (string, int, error) {
	out, err := exec.CommandContext(ctx, "/bin/sh", "-c", cmd).CombinedOutput()
	s := strings.TrimSuffix(string(out), "\n")
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return s, ee.ExitCode(), nil
	}
	if err != nil {
		return s, -1, err
	}
	// Check if context was cancelled after command completed
	if ctx.Err() != nil {
		return s, -1, ctx.Err()
	}
	return s, 0, nil
}

func (LocalShell) Protocol() string { return "local" }
func (LocalShell) Close() error     { return nil }
