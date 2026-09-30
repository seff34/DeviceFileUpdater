package transport

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"devupdater/internal/shell"
)

const (
	tIAC  = 255
	tDONT = 254
	tDO   = 253
	tWONT = 252
	tWILL = 251
	tSB   = 250
	tSE   = 240

	optEcho = 1
	optSGA  = 3
)

// iacReader strips telnet negotiation from the stream and answers it.
type iacReader struct {
	c     net.Conn
	state int
	cmd   byte
}

func (r *iacReader) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))
	for {
		n, err := r.c.Read(buf)
		out := 0
		for _, b := range buf[:n] {
			switch r.state {
			case 0:
				if b == tIAC {
					r.state = 1
				} else {
					p[out] = b
					out++
				}
			case 1:
				switch b {
				case tIAC:
					p[out] = b
					out++
					r.state = 0
				case tDO, tDONT, tWILL, tWONT:
					r.cmd, r.state = b, 2
				case tSB:
					r.state = 3
				default:
					r.state = 0
				}
			case 2:
				r.answer(r.cmd, b)
				r.state = 0
			case 3:
				if b == tIAC {
					r.state = 4
				}
			case 4:
				if b == tSE {
					r.state = 0
				} else {
					r.state = 3
				}
			}
		}
		if out > 0 || err != nil {
			return out, err
		}
	}
}

// answer accepts server echo + suppress-go-ahead and refuses everything else.
func (r *iacReader) answer(cmd, opt byte) {
	switch cmd {
	case tDO:
		if opt == optSGA {
			r.c.Write([]byte{tIAC, tWILL, opt})
		} else {
			r.c.Write([]byte{tIAC, tWONT, opt})
		}
	case tWILL:
		if opt == optEcho || opt == optSGA {
			r.c.Write([]byte{tIAC, tDO, opt})
		} else {
			r.c.Write([]byte{tIAC, tDONT, opt})
		}
	}
}

var (
	reLogin     = regexp.MustCompile(`(?i)(login|username|user name)\s*:\s*$`)
	rePassOrSh  = regexp.MustCompile(`(?i)(password\s*:\s*$)|([#$>%]\s*$)`)
	reAfterPass = regexp.MustCompile(`(?i)(login incorrect|authentication fail\w*|access denied|login failed)|(login\s*:\s*$)|([#$>%]\s*$)`)
)

type telnetSession struct {
	conn net.Conn
	m    *shell.MarkerSession
}

func DialTelnet(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
	d := net.Dialer{Timeout: opt.ConnectTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("telnet connect %s: %w", addr, err)
	}
	m := shell.NewMarkerSession(&iacReader{c: conn}, conn, "\r\n")
	lctx, cancel := context.WithTimeout(ctx, opt.ConnectTimeout+opt.CommandTimeout)
	defer cancel()
	fail := func(e error) (Session, error) { conn.Close(); return nil, e }

	if _, err := m.Expect(lctx, reLogin); err != nil {
		return fail(fmt.Errorf("telnet %s: no login prompt: %w", addr, err))
	}
	m.Send(user + "\r\n")
	got, err := m.Expect(lctx, rePassOrSh)
	if err != nil {
		return fail(fmt.Errorf("telnet %s: no password prompt: %w", addr, err))
	}
	if sm := rePassOrSh.FindStringSubmatch(got); sm[1] != "" {
		m.Send(pass + "\r\n")
		got, err = m.Expect(lctx, reAfterPass)
		if err != nil {
			return fail(fmt.Errorf("telnet %s: no shell prompt after login: %w", addr, err))
		}
		if sm := reAfterPass.FindStringSubmatch(got); sm[1] != "" || sm[2] != "" {
			return fail(fmt.Errorf("telnet %s: %w", addr, ErrAuth))
		}
	}
	if err := m.Init(lctx); err != nil {
		return fail(fmt.Errorf("telnet %s: shell init: %w", addr, err))
	}
	return &telnetSession{conn: conn, m: m}, nil
}

func (s *telnetSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	return s.m.Exec(ctx, cmd)
}
func (s *telnetSession) Protocol() string { return "telnet" }
func (s *telnetSession) Close() error     { return s.conn.Close() }
