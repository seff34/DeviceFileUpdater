package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"devupdater/internal/shell"

	"golang.org/x/crypto/ssh"
)

// syncBuffer lets stdout and stderr copy goroutines share one buffer safely.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type SSHSession struct {
	client *ssh.Client
	pty    *shell.MarkerSession // non-nil when the server refuses exec
}

func DialSSH(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
			ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
				ans := make([]string, len(qs))
				for i := range ans {
					ans[i] = pass
				}
				return ans, nil
			}),
		},
		HostKeyCallback: hostKeyCallback(opt),
		Timeout:         opt.ConnectTimeout,
	}
	d := net.Dialer{Timeout: opt.ConnectTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh connect %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(opt.ConnectTimeout))
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		if isSSHAuthError(err) {
			return nil, fmt.Errorf("ssh %s: %w", addr, ErrAuth)
		}
		// x/crypto wraps the host key callback error with %w, so ErrHostKey survives.
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	conn.SetDeadline(time.Time{})
	s := &SSHSession{client: ssh.NewClient(c, chans, reqs)}

	cctx, cancel := context.WithTimeout(ctx, opt.CommandTimeout)
	defer cancel()
	if _, code, err := s.execChannel(cctx, "true"); err == nil && code == 0 {
		return s, nil
	}
	if err := s.startPTY(cctx); err != nil {
		s.client.Close()
		return nil, fmt.Errorf("ssh %s: neither exec nor shell works: %w", addr, err)
	}
	return s, nil
}

func (s *SSHSession) startPTY(ctx context.Context) error {
	var (
		stdin  io.WriteCloser
		stdout io.Reader
	)
	err := doCtx(ctx, func() { s.client.Close() }, func() error {
		sess, err := s.client.NewSession()
		if err != nil {
			return err
		}
		if err := sess.RequestPty("vt100", 40, 1000, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
			sess.Close()
			return err
		}
		in, err := sess.StdinPipe()
		if err != nil {
			sess.Close()
			return err
		}
		out, err := sess.StdoutPipe()
		if err != nil {
			sess.Close()
			return err
		}
		if err := sess.Shell(); err != nil {
			sess.Close()
			return err
		}
		stdin, stdout = in, out
		return nil
	})
	if err != nil {
		return err
	}
	m := shell.NewMarkerSession(stdout, stdin, "\n")
	if err := m.Init(ctx); err != nil {
		m.Close()
		return err
	}
	s.pty = m
	return nil
}

// doCtx runs fn, calling abort (which must unblock fn) if ctx ends first.
// It returns ctx.Err() without waiting for fn, so fn must only publish
// results through variables read after a nil return.
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

func (s *SSHSession) execChannel(ctx context.Context, cmd string) (string, int, error) {
	// NewSession and Start wait for the device without a deadline; if ctx
	// ends first the client is closed: a device that stalls a channel open
	// or exec reply is wedged, and closing is the only way to unblock them.
	var (
		buf  syncBuffer
		sess *ssh.Session
	)
	err := doCtx(ctx, func() { s.client.Close() }, func() error {
		ss, err := s.client.NewSession()
		if err != nil {
			return err
		}
		ss.Stdout, ss.Stderr = &buf, &buf
		if err := ss.Start(cmd); err != nil {
			ss.Close()
			return err
		}
		sess = ss
		return nil
	})
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case err := <-done:
		code := 0
		if err != nil {
			var ee *ssh.ExitError
			if !errors.As(err, &ee) {
				return buf.String(), -1, err
			}
			code = ee.ExitStatus()
		}
		return strings.TrimSuffix(buf.String(), "\n"), code, nil
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		return "", -1, ctx.Err()
	}
}

func (s *SSHSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	if s.pty != nil {
		return s.pty.Exec(ctx, cmd)
	}
	return s.execChannel(ctx, cmd)
}

func (s *SSHSession) Client() *ssh.Client { return s.client }
func (s *SSHSession) Protocol() string    { return "ssh" }
func (s *SSHSession) Close() error {
	if s.pty != nil {
		s.pty.Close()
	}
	return s.client.Close()
}

// isSSHAuthError reports credential rejection. OpenSSH without keyboard-interactive
// answers our keyboard-interactive attempt with USERAUTH_FAILURE (type 51), which
// x/crypto surfaces as "unexpected message type 51 (expected 60)" instead of the
// usual "unable to authenticate".
func isSSHAuthError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "unexpected message type 51")
}
