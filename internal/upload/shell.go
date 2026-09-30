package upload

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

const (
	// maxPayload caps the data carried by one command.
	maxPayload = 700
	// maxCmdLen is the device limit for one command line; marginLen is headroom
	// for shell/transport overhead (the marker echo is sent on its own line).
	maxCmdLen = 1024
	marginLen = 64
	minBudget = 64
)

var errPathTooLong = errors.New("remote path too long for shell upload")

// budget returns how many payload bytes fit next to fixedLen bytes of other command text.
func budget(fixedLen int) (int, error) {
	n := min(maxPayload, maxCmdLen-marginLen-fixedLen)
	if n < minBudget {
		return 0, errPathTooLong
	}
	return n, nil
}

type shellBase64 struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellBase64(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellBase64{s: s, timeout: cmdTimeout}
}

func (u *shellBase64) Name() string       { return "shell-base64" }
func (u *shellBase64) perCommandTimeout() {}

func (u *shellBase64) Upload(ctx context.Context, data []byte, remote string) error {
	b64 := shell.Quote(remote + ".b64")
	dst := shell.Quote(remote)
	truncate := "true > " + b64
	// Decode and cleanup are separate commands so each stays short with long paths.
	decode := "base64 -d < " + b64 + " > " + dst
	// "printf '%s' '" + chunk + "' >> " + b64
	n, err := budget(len("printf '%s' '' >> ") + len(b64))
	if err != nil {
		return err
	}
	if len(truncate) > maxCmdLen-marginLen || len(decode) > maxCmdLen-marginLen {
		return errPathTooLong
	}
	// From here on the .b64 file may exist; any failure gets one best-effort cleanup that
	// still runs after ctx cancellation.
	fail := func(err error) error {
		_ = run(context.WithoutCancel(ctx), u.s, u.timeout, "cleanup", "rm -f "+b64)
		return err
	}
	if err := run(ctx, u.s, u.timeout, "truncate", truncate); err != nil {
		return fail(err)
	}
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 0 {
		k := min(n, len(enc))
		if err := run(ctx, u.s, u.timeout, "write chunk", "printf '%s' "+shell.Quote(enc[:k])+" >> "+b64); err != nil {
			return fail(err)
		}
		enc = enc[k:]
	}
	if err := run(ctx, u.s, u.timeout, "base64 decode", decode); err != nil {
		return fail(err)
	}
	return run(ctx, u.s, u.timeout, "remove temp", "rm -f "+b64)
}

type shellPrintf struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellPrintf(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellPrintf{s: s, timeout: cmdTimeout}
}

func (u *shellPrintf) Name() string       { return "shell-printf" }
func (u *shellPrintf) perCommandTimeout() {}

// printfEscape turns bytes into a printf format string that reproduces them exactly.
func printfEscape(b byte) string {
	if b >= 0x21 && b <= 0x7e && b != '\\' && b != '%' && b != '\'' && b != '-' {
		return string(b)
	}
	return fmt.Sprintf(`\%03o`, b)
}

func (u *shellPrintf) Upload(ctx context.Context, data []byte, remote string) error {
	dst := shell.Quote(remote)
	if len(data) == 0 {
		return run(ctx, u.s, u.timeout, "truncate", "true > "+dst)
	}
	// "printf '" + chunk + "' >> " + dst
	limit, err := budget(len("printf '' >> ") + len(dst))
	if err != nil {
		return err
	}
	redirect := ">"
	var chunk strings.Builder
	flush := func() error {
		err := run(ctx, u.s, u.timeout, "write chunk", "printf '"+chunk.String()+"' "+redirect+" "+dst)
		chunk.Reset()
		redirect = ">>"
		return err
	}
	for _, b := range data {
		e := printfEscape(b)
		if chunk.Len()+len(e) > limit {
			if err := flush(); err != nil {
				return err
			}
		}
		chunk.WriteString(e)
	}
	return flush()
}
