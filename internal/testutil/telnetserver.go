package testutil

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// StartFakeTelnet serves a busybox-like telnet login followed by a fake shell.
func StartFakeTelnet(t *testing.T, user, pass string, handler func(cmd string) (string, int)) string {
	t.Helper()
	return StartFakeTelnetMOTD(t, user, pass, "", handler)
}

// StartFakeTelnetMOTD is StartFakeTelnet but prints motd after a successful
// login, before the shell prompt.
func StartFakeTelnetMOTD(t *testing.T, user, pass, motd string, handler func(cmd string) (string, int)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTelnet(c, bufio.NewReader(c), user, pass, motd, handler)
		}
	}()
	return ln.Addr().String()
}

func serveTelnet(c net.Conn, r *bufio.Reader, user, pass, motd string, handler func(string) (string, int)) {
	defer c.Close()
	c.Write([]byte{255, 253, 1}) // IAC DO ECHO — client must filter and answer
	readLine := func() (string, error) {
		s, err := r.ReadString('\n')
		// drop any IAC replies (3-byte sequences) that precede text
		for strings.HasPrefix(s, "\xff") && len(s) >= 3 {
			s = s[3:]
		}
		return strings.TrimRight(s, "\r\n"), err
	}
	for {
		io.WriteString(c, "Welcome\r\nbox login: ")
		u, err := readLine()
		if err != nil {
			return
		}
		io.WriteString(c, "Password: ")
		p, err := readLine()
		if err != nil {
			return
		}
		if u == user && p == pass {
			break
		}
		io.WriteString(c, "\r\nLogin incorrect\r\n")
	}
	io.WriteString(c, "\r\n"+motd+"~ $ ")
	ServeFakeShell(r, c, true, handler)
}

// ListenFakeTelnet is StartFakeTelnet for programs without a *testing.T (the
// e2e fake fleet). A client whose first bytes are "SSH-" is dropped at once,
// so a tool that dials SSH first falls back to Telnet without a timeout.
func ListenFakeTelnet(addr, user, pass string, handler func(cmd string) (string, int)) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				r := bufio.NewReader(c)
				c.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
				if b, _ := r.Peek(4); string(b) == "SSH-" {
					c.Close()
					return
				}
				c.SetReadDeadline(time.Time{})
				serveTelnet(c, r, user, pass, "", handler)
			}()
		}
	}()
	return ln, nil
}
