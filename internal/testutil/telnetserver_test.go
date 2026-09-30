package testutil

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"
)

func TestListenFakeTelnetDropsSSHAndServesTelnet(t *testing.T) {
	ln, err := ListenFakeTelnet("127.0.0.1:0", "u", "p", func(cmd string) (string, int) { return "hi", 0 })
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	c, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	c.Write([]byte("SSH-2.0-Go\r\n"))
	c.SetReadDeadline(time.Now().Add(2 * time.Second))
	if n, err := c.Read(make([]byte, 64)); err == nil || n != 0 {
		t.Fatalf("ssh client should be dropped, got n=%d err=%v", n, err)
	}
	c.Close()

	c2, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	c2.SetReadDeadline(time.Now().Add(2 * time.Second))
	r := bufio.NewReader(c2)
	got := ""
	for !strings.HasSuffix(got, "login: ") {
		b, err := r.ReadByte()
		if err != nil {
			t.Fatalf("no login prompt, got %q: %v", got, err)
		}
		got += string(b)
	}
}
