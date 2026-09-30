// Package transport connects to devices and exposes a uniform command session.
package transport

import (
	"context"
	"errors"
	"time"
)

// Session runs shell commands on a device. Output is stdout+stderr combined.
type Session interface {
	Exec(ctx context.Context, cmd string) (string, int, error)
	Protocol() string
	Close() error
}

type Options struct {
	ConnectTimeout time.Duration
	CommandTimeout time.Duration
	StrictHostKey  bool
	KnownHostsPath string
}

// ErrAuth marks credential rejection (no point retrying the same credentials).
var ErrAuth = errors.New("authentication failed")
