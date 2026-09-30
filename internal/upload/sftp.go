package upload

import (
	"context"
	"os"
	"sync"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// doCtx runs fn, calling abort (which should unblock fn) if ctx ends first.
// It returns ctx.Err() without waiting for fn, so fn must only touch state it owns.
func doCtx(ctx context.Context, abort func(), fn func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		abort()
		return ctx.Err()
	}
}

type sftpUp struct{ c *ssh.Client }

func NewSFTP(c *ssh.Client) Uploader { return &sftpUp{c: c} }
func (u *sftpUp) Name() string       { return "sftp" }

func (u *sftpUp) Upload(ctx context.Context, data []byte, remote string) error {
	var (
		mu     sync.Mutex
		sc     *sftp.Client
		closed bool
	)
	abort := func() {
		mu.Lock()
		defer mu.Unlock()
		closed = true
		if sc != nil {
			sc.Close()
		}
	}
	return doCtx(ctx, abort, func() error {
		c, err := sftp.NewClient(u.c)
		if err != nil {
			return err
		}
		defer c.Close()
		mu.Lock()
		if closed { // ctx ended while the subsystem was starting
			mu.Unlock()
			return ctx.Err()
		}
		sc = c
		mu.Unlock()
		f, err := c.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}
