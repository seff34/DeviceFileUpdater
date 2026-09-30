package syncer

import (
	"context"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

func exists(ctx context.Context, s transport.Session, t time.Duration, p string) (bool, error) {
	out, err := exec(ctx, s, t, "[ -f "+shell.Quote(p)+" ] && echo Y || echo N")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "Y", nil
}

func readMode(ctx context.Context, s transport.Session, t time.Duration, p string) (string, bool) {
	q := shell.Quote(p)
	out, err := exec(ctx, s, t, "stat -c %a "+q+" 2>/dev/null || ls -ln "+q)
	if err != nil {
		return "", false
	}
	return parseMode(strings.TrimSpace(out))
}

// parseMode accepts `stat -c %a` output ("755") or an `ls -l` line ("-rwxr-xr-x ...").
func parseMode(s string) (string, bool) {
	if len(s) >= 3 && len(s) <= 4 && strings.Trim(s, "01234567") == "" {
		return strings.Repeat("0", 4-len(s)) + s, true
	}
	f := strings.Fields(s)
	if len(f) == 0 || len(f[0]) < 10 {
		return "", false
	}
	perm := f[0][1:10]
	digits := make([]byte, 3)
	for i := 0; i < 3; i++ {
		v := 0
		t := perm[i*3 : i*3+3]
		if t[0] == 'r' {
			v += 4
		}
		if t[1] == 'w' {
			v += 2
		}
		if t[2] == 'x' || t[2] == 's' || t[2] == 't' {
			v++
		}
		if strings.Trim(t, "rwxsStT-") != "" {
			return "", false
		}
		digits[i] = byte('0' + v)
	}
	return "0" + string(digits), true
}
