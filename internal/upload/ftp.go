package upload

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"

	"github.com/jlaffaye/ftp"
)

type FTPCreds struct{ Host, User, Pass string }

// ftpPort is a var so tests can point the uploader at a local fake.
var ftpPort = "21"

type ftpUp struct {
	s       transport.Session
	cr      FTPCreds
	timeout time.Duration
}

func NewFTP(s transport.Session, cr FTPCreds, cmdTimeout time.Duration) Uploader {
	if cmdTimeout <= 0 {
		cmdTimeout = defaultCmdTimeout
	}
	return &ftpUp{s: s, cr: cr, timeout: cmdTimeout}
}

func (u *ftpUp) Name() string { return "ftp" }

// connSet tracks every connection the ftp client opens so a cancelled
// upload can close them all and unblock the client.
type connSet struct {
	mu     sync.Mutex
	conns  []net.Conn
	closed bool
}

func (cs *connSet) add(c net.Conn) net.Conn {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if cs.closed {
		c.Close()
		return c
	}
	cs.conns = append(cs.conns, c)
	return c
}

func (cs *connSet) closeAll() {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.closed = true
	for _, c := range cs.conns {
		c.Close()
	}
}

func (u *ftpUp) Upload(ctx context.Context, data []byte, remote string) error {
	cs := &connSet{}
	defer cs.closeAll()
	dialer := net.Dialer{Timeout: u.timeout}
	dial := func(network, addr string) (net.Conn, error) {
		c, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return cs.add(c), nil
	}
	err := doCtx(ctx, cs.closeAll, func() error {
		c, err := ftp.Dial(net.JoinHostPort(u.cr.Host, ftpPort),
			ftp.DialWithDialFunc(dial), ftp.DialWithTimeout(u.timeout))
		if err != nil {
			return err
		}
		defer c.Quit()
		if err := c.Login(u.cr.User, u.cr.Pass); err != nil {
			return err
		}
		return c.Stor(remote, bytes.NewReader(data))
	})
	if err != nil {
		return err
	}
	// The ftpd root may not be "/": confirm the file really landed at remote.
	cctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()
	out, code, err := u.s.Exec(cctx, "wc -c < "+shell.Quote(remote))
	if err != nil {
		return err
	}
	if n, _ := strconv.Atoi(strings.TrimSpace(out)); code != 0 || n != len(data) {
		return fmt.Errorf("ftp: file not found at %s after upload (ftpd root is not /?)", remote)
	}
	return nil
}
