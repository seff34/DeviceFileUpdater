package transport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devupdater/internal/workspace"
)

// Package vars for dependency injection in tests.
var (
	retryDelay = time.Second
	dialSSH    = DialSSH
	dialTelnet = DialTelnet
)

// Dial connects with SSH, falling back to Telnet, retrying once on connection errors.
func Dial(ctx context.Context, d workspace.Device, opt Options) (Session, error) {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		var s Session
		s, err = dialOnce(ctx, d, opt)
		if err == nil {
			return s, nil
		}
		if errors.Is(err, ErrAuth) || ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, err
}

func dialOnce(ctx context.Context, d workspace.Device, opt Options) (Session, error) {
	s, sshErr := dialSSH(ctx, d.Addr(22), d.Username, d.Password, opt)
	if sshErr == nil {
		return s, nil
	}
	s, telErr := dialTelnet(ctx, d.Addr(23), d.Username, d.Password, opt)
	if telErr == nil {
		return s, nil
	}
	if errors.Is(sshErr, ErrAuth) || errors.Is(telErr, ErrAuth) {
		return nil, fmt.Errorf("%w (ssh: %w; telnet: %w)", ErrAuth, sshErr, telErr)
	}
	return nil, fmt.Errorf("ssh: %w; telnet: %w", sshErr, telErr)
}
