package upload

import (
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/transport"

	"golang.org/x/crypto/ssh"
)

// Select returns candidate uploaders in preference order; shell-printf is always last.
func Select(s transport.Session, c probe.Caps, cr FTPCreds, cmdTimeout time.Duration) []Uploader {
	var us []Uploader
	if cs, ok := s.(interface{ Client() *ssh.Client }); ok {
		if cl := cs.Client(); cl != nil {
			if c.SFTP {
				us = append(us, NewSFTP(cl))
			}
			if c.Has("scp") {
				us = append(us, NewSCP(cl))
			}
		}
	}
	if c.FTP {
		us = append(us, NewFTP(s, cr, cmdTimeout))
	}
	if c.Has("base64") {
		us = append(us, NewShellBase64(s, cmdTimeout))
	}
	return append(us, NewShellPrintf(s, cmdTimeout))
}
