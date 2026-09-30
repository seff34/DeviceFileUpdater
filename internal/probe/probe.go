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
		if name, ok := strings.CutPrefix(strings.TrimSpace(line), "HAVE "); ok {
			c.Tools[name] = true
		}
	}
	if cs, ok := s.(interface{ Client() *ssh.Client }); ok {
		if sc, err := sftp.NewClient(cs.Client()); err == nil {
			c.SFTP = true
			sc.Close()
		}
	}
	if timeout > 3*time.Second {
		timeout = 3 * time.Second
	}
	if conn, err := net.DialTimeout("tcp", net.JoinHostPort(ftpHost, "21"), timeout); err == nil {
		c.FTP = true
		conn.Close()
	}
	return c, nil
}
