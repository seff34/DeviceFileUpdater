package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FakeSSH struct {
	User, Pass string
	NoExec     bool // reject "exec" requests, forcing PTY shell fallback
	Handler    func(cmd string) (string, int)
}

func (f FakeSSH) Start(t *testing.T) string {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if c.User() == f.User && string(p) == f.Pass {
				return nil, nil
			}
			return nil, fmt.Errorf("denied")
		},
	}
	cfg.AddHostKey(signer)
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
			go f.serve(c, cfg)
		}
	}()
	return ln.Addr().String()
}

func (f FakeSSH) serve(c net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go f.session(ch, creqs)
	}
}

func (f FakeSSH) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			req.Reply(true, nil)
		case "exec":
			if f.NoExec {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			n := binary.BigEndian.Uint32(req.Payload[:4])
			out, code := f.Handler(string(req.Payload[4 : 4+n]))
			if out != "" {
				ch.Write([]byte(out + "\n"))
			}
			status := make([]byte, 4)
			binary.BigEndian.PutUint32(status, uint32(code))
			ch.SendRequest("exit-status", false, status)
			return
		case "shell":
			req.Reply(true, nil)
			go ServeFakeShell(ch, ch, true, f.Handler)
		case "subsystem":
			n := binary.BigEndian.Uint32(req.Payload[:4])
			if string(req.Payload[4:4+n]) != "sftp" {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			srv, err := sftp.NewServer(ch)
			if err != nil {
				return
			}
			srv.Serve()
			return
		default:
			req.Reply(false, nil)
		}
	}
}
