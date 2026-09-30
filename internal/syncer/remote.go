package syncer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

// Target kinds reported by probeType.
const (
	kindFile    = "F"
	kindAbsent  = "N"
	kindSymlink = "L"
	kindDir     = "D"
	kindOther   = "O"
)

// probeType classifies the remote path without following symlinks.
func probeType(ctx context.Context, s transport.Session, t time.Duration, p string) (string, error) {
	q := shell.Quote(p)
	out, err := exec(ctx, s, t, "if [ -L "+q+" ]; then echo L; elif [ -d "+q+" ]; then echo D; elif [ -f "+q+" ]; then echo F; elif [ -e "+q+" ]; then echo O; else echo N; fi")
	if err != nil {
		return "", err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	switch last {
	case kindFile, kindAbsent, kindSymlink, kindDir, kindOther:
		return last, nil
	}
	return "", fmt.Errorf("unexpected output %q", last)
}

func readMode(ctx context.Context, s transport.Session, t time.Duration, p string) (string, bool) {
	q := shell.Quote(p)
	out, err := exec(ctx, s, t, "stat -c %a "+q+" 2>/dev/null || ls -ln "+q)
	if err != nil {
		return "", false
	}
	return parseMode(strings.TrimSpace(out))
}

// parseMode accepts `stat -c %a` output (1-4 octal digits) or an `ls -l` line ("-rwxr-xr-x ...").
func parseMode(s string) (string, bool) {
	if len(s) >= 1 && len(s) <= 4 && strings.Trim(s, "01234567") == "" {
		return strings.Repeat("0", 4-len(s)) + s, true
	}
	f := strings.Fields(s)
	if len(f) == 0 || len(f[0]) < 10 {
		return "", false
	}
	perm := f[0][1:10]
	digits := make([]byte, 3)
	special := 0
	for i := 0; i < 3; i++ {
		v := 0
		t := perm[i*3 : i*3+3]
		if strings.Trim(t, "rwxsStT-") != "" || (t[0] != 'r' && t[0] != '-') || (t[1] != 'w' && t[1] != '-') {
			return "", false
		}
		if t[0] == 'r' {
			v += 4
		}
		if t[1] == 'w' {
			v += 2
		}
		switch t[2] {
		case 'x':
			v++
		case 's', 'S':
			if i == 2 {
				return "", false
			}
			special |= 4 >> i
			if t[2] == 's' {
				v++
			}
		case 't', 'T':
			if i != 2 {
				return "", false
			}
			special |= 1
			if t[2] == 't' {
				v++
			}
		case '-':
		default:
			return "", false
		}
		digits[i] = byte('0' + v)
	}
	return string(rune('0'+special)) + string(digits), true
}
