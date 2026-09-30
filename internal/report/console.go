package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"devupdater/internal/model"
)

// clean makes device-supplied text safe for a terminal: control characters
// (except tab) and invalid UTF-8 bytes become visible Go-style escapes, so an
// error message cannot move the cursor, recolour or retitle the console.
func clean(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, n := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && n == 1:
			fmt.Fprintf(&b, `\x%02x`, s[i])
		case r != '\t' && unicode.IsControl(r):
			q := strconv.QuoteRune(r)
			b.WriteString(q[1 : len(q)-1])
		default:
			b.WriteString(s[i : i+n])
		}
		i += n
	}
	return b.String()
}

func PrintConsole(w io.Writer, r model.RunResult) {
	c := Summarize(r)
	mode := ""
	if r.DryRun {
		mode = " (dry-run)"
	}
	fmt.Fprintf(w, "\nRun %s%s: %d devices, %d failed\n", r.ID, mode, c.Devices, c.DevicesFailed)
	for _, s := range statusOrder {
		if n := c.ByStatus[s]; n > 0 {
			fmt.Fprintf(w, "  %-13s %d\n", s, n)
		}
	}
	for _, d := range r.Devices {
		if !d.Failed() {
			continue
		}
		fmt.Fprintf(w, "FAILED %s", clean(d.Host))
		if d.Error != "" {
			fmt.Fprintf(w, ": %s", clean(d.Error))
		}
		fmt.Fprintln(w)
		for _, f := range d.Files {
			if f.Status == model.Failed && d.Error == "" {
				fmt.Fprintf(w, "  %s: %s\n", clean(f.Remote), clean(f.Error))
			}
		}
		if d.Post != nil && (d.Post.ExitCode != 0 || d.Post.Error != "") {
			fmt.Fprintf(w, "  post command exit %d %s\n", d.Post.ExitCode, clean(d.Post.Error))
		}
	}
}
