// Package upload writes file content to a device path using the best available method.
package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"devupdater/internal/transport"
)

// defaultCmdTimeout applies when a caller passes a zero or negative timeout.
const defaultCmdTimeout = 30 * time.Second

type Uploader interface {
	Name() string
	// Upload creates or truncates remote and writes data to it.
	Upload(ctx context.Context, data []byte, remote string) error
}

// minUploadRate (bytes/s) sizes each network upload attempt's deadline:
// cmdTimeout plus the size at this rate. A stalled sftp/scp/ftp transfer is
// abandoned (and the next method tried) instead of hanging the device worker.
const minUploadRate = 32 << 10

// transferTimeout is the deadline for one network upload attempt of size bytes.
func transferTimeout(cmdTimeout time.Duration, size int) time.Duration {
	if cmdTimeout <= 0 {
		cmdTimeout = defaultCmdTimeout
	}
	return cmdTimeout + time.Duration(size)*time.Second/minUploadRate
}

// perCommand marks uploaders that send many short commands, each already
// bounded by cmdTimeout. They get no whole-upload deadline: on a slow but
// steady link (the weak telnet devices they exist for) a large file may take
// far longer than size/minUploadRate.
type perCommand interface{ perCommandTimeout() }

// Chain tries uploaders in order. One that fails is skipped for the rest of the device run.
type Chain struct {
	list       []Uploader
	cmdTimeout time.Duration
	mu         sync.Mutex
	broken     map[string]bool
}

func NewChain(us []Uploader, cmdTimeout time.Duration) *Chain {
	return &Chain{list: us, cmdTimeout: cmdTimeout, broken: map[string]bool{}}
}

func (c *Chain) isBroken(name string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.broken[name]
}

func (c *Chain) Primary() string {
	for _, u := range c.list {
		if !c.isBroken(u.Name()) {
			return u.Name()
		}
	}
	return ""
}

func (c *Chain) Upload(ctx context.Context, data []byte, remote string) (string, error) {
	var errs []string
	for _, u := range c.list {
		if c.isBroken(u.Name()) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		err := c.attempt(ctx, u, data, remote)
		if err == nil {
			return u.Name(), nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		c.mu.Lock()
		c.broken[u.Name()] = true
		c.mu.Unlock()
		errs = append(errs, u.Name()+": "+err.Error())
	}
	if len(errs) == 0 {
		return "", errors.New("no upload method available")
	}
	return "", fmt.Errorf("all upload methods failed: %s", strings.Join(errs, "; "))
}

// attempt runs one uploader. All but the per-command (shell) uploaders get
// their own size-scaled deadline, so a stall fails over to the next method
// without eating its time.
func (c *Chain) attempt(ctx context.Context, u Uploader, data []byte, remote string) error {
	if _, ok := u.(perCommand); ok {
		return u.Upload(ctx, data, remote)
	}
	limit := transferTimeout(c.cmdTimeout, len(data))
	uctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	err := u.Upload(uctx, data, remote)
	if err != nil && uctx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
		err = fmt.Errorf("timed out after %v: %w", limit, err)
	}
	return err
}

// run executes cmd with its own timeout; a non-zero exit becomes an error labelled with label.
func run(ctx context.Context, s transport.Session, timeout time.Duration, label, cmd string) error {
	if timeout <= 0 {
		timeout = defaultCmdTimeout
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, code, err := s.Exec(c, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if code != 0 {
		return fmt.Errorf("%s: exit %d: %s", label, code, strings.TrimSpace(out))
	}
	return nil
}
