package transport

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

var knownHostsMu sync.Mutex

// checkKnownHost implements trust-on-first-use against a simple "host type key" file.
func checkKnownHost(path, host string, key ssh.PublicKey) error {
	knownHostsMu.Lock()
	defer knownHostsMu.Unlock()
	want := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key)))
	if f, err := os.Open(path); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			h, k, ok := strings.Cut(sc.Text(), " ")
			if ok && h == host {
				f.Close()
				if k != want {
					return fmt.Errorf("host key for %s changed (remove its line from %s if the device was reflashed)", host, path)
				}
				return nil
			}
		}
		f.Close()
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "%s %s\n", host, want)
	return err
}

func hostKeyCallback(opt Options) ssh.HostKeyCallback {
	if !opt.StrictHostKey {
		return ssh.InsecureIgnoreHostKey()
	}
	return func(hostname string, _ net.Addr, key ssh.PublicKey) error {
		return checkKnownHost(opt.KnownHostsPath, hostname, key)
	}
}
