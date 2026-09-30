package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"

	"devupdater/internal/shell"

	"golang.org/x/crypto/ssh"
)

type scpUp struct{ c *ssh.Client }

func NewSCP(c *ssh.Client) Uploader { return &scpUp{c: c} }
func (u *scpUp) Name() string       { return "scp" }

// Upload speaks the scp sink protocol against `scp -t <remote>`.
func (u *scpUp) Upload(ctx context.Context, data []byte, remote string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sess, err := u.c.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := sess.StdoutPipe()
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
	return doCtx(ctx, func() { sess.Close() }, func() error {
		if err := sess.Start("scp -t " + shell.Quote(remote)); err != nil {
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
		return sess.Wait()
	})
}
