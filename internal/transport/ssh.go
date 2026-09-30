package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, fmt.Errorf("ssh %s: %w", addr, ErrAuth)
		}
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
	sess, err := s.client.NewSession()
	if err != nil {
		return err
	}
	if err := sess.RequestPty("vt100", 40, 1000, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		return err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	if err := sess.Shell(); err != nil {
		return err
	}
	m := shell.NewMarkerSession(stdout, stdin, "\n")
	if err := m.Init(ctx); err != nil {
		return err
	}
	s.pty = m
	return nil
}

func (s *SSHSession) execChannel(ctx context.Context, cmd string) (string, int, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()
	var buf syncBuffer
	sess.Stdout, sess.Stderr = &buf, &buf
	if err := sess.Start(cmd); err != nil {
		return "", -1, err
	}
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
func (s *SSHSession) Close() error        { return s.client.Close() }
