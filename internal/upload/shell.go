package upload

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

// Keeps every command line under the 1024-byte limit including quoting and path.
const maxPayload = 700

type shellBase64 struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellBase64(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellBase64{s: s, timeout: cmdTimeout}
}

func (u *shellBase64) Name() string { return "shell-base64" }

func (u *shellBase64) Upload(ctx context.Context, data []byte, remote string) error {
	b64 := shell.Quote(remote + ".b64")
	dst := shell.Quote(remote)
	if err := run(ctx, u.s, u.timeout, "truncate", ": > "+b64); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 0 {
		n := min(maxPayload, len(enc))
		if err := run(ctx, u.s, u.timeout, "write chunk", "printf '%s' "+shell.Quote(enc[:n])+" >> "+b64); err != nil {
			// Best-effort cleanup that still runs after ctx cancellation; its error is secondary.
			_ = run(context.WithoutCancel(ctx), u.s, u.timeout, "cleanup", "rm -f "+b64)
			return err
		}
		enc = enc[n:]
	}
	return run(ctx, u.s, u.timeout, "base64 decode",
		fmt.Sprintf("if base64 -d < %s > %s; then rm -f %s; else rm -f %s; false; fi", b64, dst, b64, b64))
}

type shellPrintf struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellPrintf(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellPrintf{s: s, timeout: cmdTimeout}
}

func (u *shellPrintf) Name() string { return "shell-printf" }

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
		return run(ctx, u.s, u.timeout, "truncate", ": > "+dst)
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
		if chunk.Len()+len(e) > maxPayload {
			if err := flush(); err != nil {
				return err
			}
		}
		chunk.WriteString(e)
	}
	return flush()
}
