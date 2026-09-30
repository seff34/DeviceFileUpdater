package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"sync"

	"devupdater/internal/shell"

	"golang.org/x/crypto/ssh"
)

type scpUp struct{ c *ssh.Client }

func NewSCP(c *ssh.Client) Uploader { return &scpUp{c: c} }
func (u *scpUp) Name() string       { return "scp" }

// Upload speaks the scp sink protocol against `scp -t <remote>`.
func (u *scpUp) Upload(ctx context.Context, data []byte, remote string) error {
	var (
		mu      sync.Mutex
		sess    *ssh.Session
		aborted bool
	)
	abort := func() {
		mu.Lock()
		defer mu.Unlock()
		aborted = true
		if sess != nil {
			sess.Close()
		} else {
			u.c.Close() // a stalled channel open only unblocks when the client closes
		}
	}
	return doCtx(ctx, abort, func() error {
		s, err := u.c.NewSession()
		if err != nil {
			return err
		}
		defer s.Close()
		mu.Lock()
		if aborted { // ctx ended while the channel was opening
			mu.Unlock()
			return ctx.Err()
		}
		sess = s
		mu.Unlock()
		stdin, err := s.StdinPipe()
		if err != nil {
			return err
		}
		stdoutPipe, err := s.StdoutPipe()
		if err != nil {
			return err
		}
		stdout := bufio.NewReader(stdoutPipe)
		ack := func() error {
			b, err := stdout.ReadByte()
			if err != nil {
				return err
			}
			if b != 0 {
				msg, _ := stdout.ReadString('\n')
				return fmt.Errorf("scp: %s", msg)
			}
			return nil
		}
		if err := s.Start("scp -t " + shell.Quote(remote)); err != nil {
			return err
		}
		if err := ack(); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdin, "C0644 %d %s\n", len(data), path.Base(remote)); err != nil {
			return err
		}
		if err := ack(); err != nil {
			return err
		}
		if _, err := stdin.Write(data); err != nil {
			return err
		}
		if _, err := stdin.Write([]byte{0}); err != nil {
			return err
		}
		if err := ack(); err != nil {
			return err
		}
		if err := stdin.Close(); err != nil {
			return err
		}
		io.Copy(io.Discard, stdout)
		return s.Wait()
	})
}
