package shell

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"sync"
)

// MarkerSession runs commands on an interactive shell reachable only as a byte stream.
type MarkerSession struct {
	mu      sync.Mutex
	w       io.Writer
	newline string
	nonce   string
	n       int
	in      chan []byte
	readErr error
	buf     []byte
	broken  error
}

func NewMarkerSession(r io.Reader, w io.Writer, newline string) *MarkerSession {
	nb := make([]byte, 4)
	rand.Read(nb)
	m := &MarkerSession{w: w, newline: newline, nonce: hex.EncodeToString(nb), in: make(chan []byte, 64)}
	go m.pump(r)
	return m
}

func (m *MarkerSession) pump(r io.Reader) {
	b := make([]byte, 4096)
	for {
		n, err := r.Read(b)
		if n > 0 {
			c := make([]byte, n)
			copy(c, b[:n])
			m.in <- c
		}
		if err != nil {
			m.readErr = err
			close(m.in)
			return
		}
	}
}

// readMore appends the next chunk (CR and NUL removed) or fails on ctx/EOF.
func (m *MarkerSession) readMore(ctx context.Context) error {
	select {
	case b, ok := <-m.in:
		if !ok {
			if m.readErr != nil {
				return fmt.Errorf("connection closed: %w", m.readErr)
			}
			return io.EOF
		}
		b = bytes.ReplaceAll(b, []byte{'\r'}, nil)
		b = bytes.ReplaceAll(b, []byte{0}, nil)
		m.buf = append(m.buf, b...)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MarkerSession) Send(s string) error {
	_, err := io.WriteString(m.w, s)
	return err
}

func (m *MarkerSession) Expect(ctx context.Context, re *regexp.Regexp) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		if loc := re.FindIndex(m.buf); loc != nil {
			out := string(m.buf[:loc[1]])
			m.buf = m.buf[loc[1]:]
			return out, nil
		}
		if err := m.readMore(ctx); err != nil {
			return string(m.buf), err
		}
	}
}

// Init clears prompts and disables echo so later output is clean.
func (m *MarkerSession) Init(ctx context.Context) error {
	_, _, err := m.Exec(ctx, "PS1=''; PS2=''; stty -echo 2>/dev/null; true")
	return err
}

func (m *MarkerSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken != nil {
		return "", -1, fmt.Errorf("session out of sync after earlier error: %w", m.broken)
	}
	m.n++
	tag := fmt.Sprintf("__DU_%s_%d_", m.nonce, m.n)
	echoed := []byte(tag + "$?__\"")
	re := regexp.MustCompile(regexp.QuoteMeta(tag) + `(\d+)__`)
	if _, err := io.WriteString(m.w, cmd+`; echo "`+tag+`$?__"`+m.newline); err != nil {
		m.broken = err
		return "", -1, err
	}
	for {
		if loc := re.FindSubmatchIndex(m.buf); loc != nil {
			out := m.buf[:loc[0]]
			code, _ := strconv.Atoi(string(m.buf[loc[2]:loc[3]]))
			rest := m.buf[loc[1]:]
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
			m.buf = append([]byte(nil), rest...)
			// Drop the echoed command line (and any prompt before it) if the tty echoed.
			if i := bytes.LastIndex(out, echoed); i >= 0 {
				out = out[i+len(echoed):]
				if j := bytes.IndexByte(out, '\n'); j >= 0 {
					out = out[j+1:]
				} else {
					out = nil
				}
			}
			return string(bytes.TrimSuffix(out, []byte{'\n'})), code, nil
		}
		if err := m.readMore(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				m.broken = err
			}
			return string(m.buf), -1, err
		}
	}
}
