package transport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func testOpts() Options {
	return Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second}
}

func TestTelnetExec(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(cmd string) (string, int) {
		if cmd == "uname" {
			return "Linux", 0
		}
		return "", 0
	})
	s, err := DialTelnet(context.Background(), addr, "dev", "pw", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Protocol() != "telnet" {
		t.Fatalf("protocol %s", s.Protocol())
	}
	out, code, err := s.Exec(context.Background(), "uname")
	if err != nil || code != 0 || out != "Linux" {
		t.Fatalf("out=%q code=%d err=%v", out, code, err)
	}
}

func TestTelnetBadPassword(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(string) (string, int) { return "", 0 })
	_, err := DialTelnet(context.Background(), addr, "dev", "wrong", testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestTelnetRefused(t *testing.T) {
	_, err := DialTelnet(context.Background(), "127.0.0.1:1", "a", "b", testOpts())
	if err == nil || errors.Is(err, ErrAuth) {
		t.Fatalf("expected connection error, got %v", err)
	}
}

func TestTelnetMOTDNotAuthFailure(t *testing.T) {
	motd := "Last failed login: Mon Jan 1\r\nwarning: permission denied for /root\r\ninvalid license\r\n"
	addr := testutil.StartFakeTelnetMOTD(t, "dev", "pw", motd, func(string) (string, int) { return "ok", 0 })
	s, err := DialTelnet(context.Background(), addr, "dev", "pw", testOpts())
	if err != nil {
		t.Fatalf("MOTD misread as failure: %v", err)
	}
	defer s.Close()
	if out, _, err := s.Exec(context.Background(), "x"); err != nil || out != "ok" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

// drain collects everything written to c until it is closed.
func drain(c net.Conn) <-chan []byte {
	ch := make(chan []byte, 1)
	go func() {
		b, _ := io.ReadAll(c)
		ch <- b
	}()
	return ch
}

func TestIACReaderByteAtATime(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	replies := drain(b)
	input := []byte("a")
	input = append(input, tIAC, tDO, optEcho)
	input = append(input, 'b')
	input = append(input, tIAC, tWILL, optSGA)
	input = append(input, tIAC, tSB, 24, 1, tIAC, tSE)
	input = append(input, 'c')
	input = append(input, tIAC, tIAC)
	input = append(input, 'd')
	go func() {
		for _, x := range input {
			b.Write([]byte{x})
		}
	}()
	r := &iacReader{c: a}
	want := []byte{'a', 'b', 'c', 0xFF, 'd'}
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("data %v want %v", got, want)
	}
	a.Close()
	b.Close()
	wantReplies := []byte{tIAC, tWONT, optEcho, tIAC, tDO, optSGA}
	if got := <-replies; !bytes.Equal(got, wantReplies) {
		t.Fatalf("replies %v want %v", got, wantReplies)
	}
}

func TestIACReaderNegotiationOnlyDoesNotReturnEmpty(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	drain(b)
	go func() {
		b.Write([]byte{tIAC, tDO, optEcho})
		time.Sleep(50 * time.Millisecond)
		b.Write([]byte("x"))
	}()
	r := &iacReader{c: a}
	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if err != nil || n != 1 || buf[0] != 'x' {
		t.Fatalf("n=%d err=%v buf=%v", n, err, buf[:n])
	}
}

// busybox ash appends a cursor-position query (ESC[6n) after its prompt; it
// must not reach the prompt matcher, even when split across reads.
func TestIACReaderStripsANSIEscapes(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	drain(b)
	go func() {
		for _, x := range []byte("ab:~$ \x1b[6n\x1b[1;32mX\x1b7Y") {
			b.Write([]byte{x})
		}
	}()
	r := &iacReader{c: a}
	want := []byte("ab:~$ XY")
	got := make([]byte, len(want))
	if _, err := io.ReadFull(r, got); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q want %q", got, want)
	}
}
