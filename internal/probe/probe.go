// Package probe discovers which tools a device offers.
package probe

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"devupdater/internal/transport"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

var Tools = []string{"sha256sum", "md5sum", "base64", "od", "hexdump", "printf", "stat", "scp"}
var ftpPort = "21"

// toolMap is a set for fast lookups during parsing.
var toolMap map[string]bool

func init() {
	toolMap = make(map[string]bool)
	for _, t := range Tools {
		toolMap[t] = true
	}
}

type Caps struct {
	Tools map[string]bool
	SFTP  bool
	FTP   bool
}

func (c Caps) Has(name string) bool { return c.Tools[name] }

func (c Caps) List() []string {
	var out []string
	for t, ok := range c.Tools {
		if ok {
			out = append(out, t)
		}
	}
	if c.SFTP {
		out = append(out, "sftp")
	}
	if c.FTP {
		out = append(out, "ftp")
	}
	sort.Strings(out)
	return out
}

func Probe(ctx context.Context, s transport.Session, ftpHost string, timeout time.Duration) (Caps, error) {
	c := Caps{Tools: map[string]bool{}}
	cmd := "for c in " + strings.Join(Tools, " ") +
		"; do (command -v $c || which $c || type $c) >/dev/null 2>&1 && echo \"HAVE $c\"; done; true"
	out, _, err := s.Exec(ctx, cmd)
	if err != nil {
		return c, err
	}
	for _, line := range strings.Split(out, "\n") {
		// Accept "HAVE <name>" after prompt noise (e.g. "~ # HAVE sha256sum")
		idx := strings.Index(line, "HAVE ")
		if idx == -1 {
			continue
		}
		name := strings.TrimSpace(line[idx+5:])
		// Only record if name is in Tools
		if toolMap[name] {
			c.Tools[name] = true
		}
	}

	// Clamp timeout: if <= 0 or > 3s, use 3s
	if timeout <= 0 || timeout > 3*time.Second {
		timeout = 3 * time.Second
	}

	// Check SFTP with context and timeout
	if cs, ok := s.(interface{ Client() *ssh.Client }); ok {
		if cl := cs.Client(); cl != nil {
			sftpOk := make(chan bool, 1)
			go func() {
				sc, err := sftp.NewClient(cl)
				if err == nil {
					sftpOk <- true
					sc.Close()
				} else {
					sftpOk <- false
				}
			}()
			// Wait for SFTP check, context done, or timeout
			select {
			case ok := <-sftpOk:
				c.SFTP = ok
			case <-ctx.Done():
				c.SFTP = false
			case <-time.After(timeout):
				c.SFTP = false
			}
		}
	}

	// Check FTP with context
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(ftpHost, ftpPort))
	if err == nil {
		c.FTP = true
		conn.Close()
	}

	return c, nil
}
