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
	// Check context error first, before interpreting other errors
	if err != nil {
		if ctx.Err() != nil {
			return s, -1, ctx.Err()
		}
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return s, ee.ExitCode(), nil
		}
		return s, -1, err
	}
	return s, 0, nil
}

func (LocalShell) Protocol() string { return "local" }
func (LocalShell) Close() error     { return nil }
