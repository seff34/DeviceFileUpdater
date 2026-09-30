package testutil

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
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
			go serveTelnet(c, user, pass, motd, handler)
		}
	}()
	return ln.Addr().String()
}

func serveTelnet(c net.Conn, user, pass, motd string, handler func(string) (string, int)) {
	defer c.Close()
	c.Write([]byte{255, 253, 1}) // IAC DO ECHO — client must filter and answer
	r := bufio.NewReader(c)
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
