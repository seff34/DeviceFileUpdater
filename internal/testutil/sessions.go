package testutil

import (
	"context"
	"sync"
)

// FakeSession answers commands from Handler and records them.
type FakeSession struct {
	Proto   string
	Handler func(cmd string) (string, int)
	mu      sync.Mutex
	Cmds    []string
}

func (f *FakeSession) Exec(_ context.Context, cmd string) (string, int, error) {
	f.mu.Lock()
	f.Cmds = append(f.Cmds, cmd)
	f.mu.Unlock()
	out, code := f.Handler(cmd)
	return out, code, nil
}

func (f *FakeSession) Protocol() string {
	if f.Proto == "" {
		return "telnet"
	}
	return f.Proto
}

func (f *FakeSession) Close() error { return nil }
