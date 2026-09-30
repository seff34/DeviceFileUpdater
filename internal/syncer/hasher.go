// Package syncer decides what to do with each file on a device and performs the write.
package syncer

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

// Hasher yields a digest of a remote file comparable with Local(data).
type Hasher interface {
	Name() string
	Remote(ctx context.Context, path string) (string, error)
	Local(data []byte) string
}

func SelectHasher(s transport.Session, c probe.Caps, cmdTimeout time.Duration) Hasher {
	switch {
	case c.Has("sha256sum"):
		return &sumHasher{s, cmdTimeout, "sha256sum"}
	case c.Has("md5sum"):
		return &sumHasher{s, cmdTimeout, "md5sum"}
	case c.Has("base64"):
		return &readback{s, cmdTimeout, "base64"}
	case c.Has("od"):
		return &readback{s, cmdTimeout, "od"}
	case c.Has("hexdump"):
		return &readback{s, cmdTimeout, "hexdump"}
	}
	return nil
}

// defaultCmdTimeout applies when a caller passes a zero or negative timeout.
const defaultCmdTimeout = 30 * time.Second

func exec(ctx context.Context, s transport.Session, timeout time.Duration, cmd string) (string, error) {
	if timeout <= 0 {
		timeout = defaultCmdTimeout
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, code, err := s.Exec(c, cmd)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out))
	}
	return out, nil
}

type sumHasher struct {
	s       transport.Session
	timeout time.Duration
	tool    string
}

func (h *sumHasher) Name() string { return h.tool }

func (h *sumHasher) Remote(ctx context.Context, p string) (string, error) {
	out, err := exec(ctx, h.s, h.timeout, h.tool+" "+shell.Quote(p))
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.tool, err)
	}
	f := strings.Fields(out)
	if len(f) == 0 {
		return "", fmt.Errorf("%s: empty output", h.tool)
	}
	return strings.ToLower(f[0]), nil
}

func (h *sumHasher) Local(data []byte) string {
	if h.tool == "md5sum" {
		s := md5.Sum(data)
		return hex.EncodeToString(s[:])
	}
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// readback pulls the whole file as text and hashes it on the host.
type readback struct {
	s       transport.Session
	timeout time.Duration
	tool    string
}

func (h *readback) Name() string { return "readback-" + h.tool }

func (h *readback) Local(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func (h *readback) Remote(ctx context.Context, p string) (string, error) {
	q := shell.Quote(p)
	var cmd string
	switch h.tool {
	case "base64":
		cmd = "base64 < " + q
	case "od":
		cmd = "od -An -v -tx1 " + q
	default:
		cmd = `hexdump -v -e '/1 "%02x"' ` + q
	}
	out, err := exec(ctx, h.s, h.timeout, cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.Name(), err)
	}
	text := strings.Join(strings.Fields(out), "")
	var data []byte
	if h.tool == "base64" {
		data, err = base64.StdEncoding.DecodeString(text)
	} else {
		data, err = hex.DecodeString(text)
	}
	if err != nil {
		return "", fmt.Errorf("%s: decode: %w", h.Name(), err)
	}
	return h.Local(data), nil
}
