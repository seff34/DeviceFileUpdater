// Package testutil holds fakes shared by tests across packages.
package testutil

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var markerLine = regexp.MustCompile(`^echo "(__DU_[0-9a-f]+_\d+_)\$\?__"$`)

// ServeFakeShell imitates an interactive sh that speaks the marker protocol:
// command line(s) followed by an `echo "<tag>$?__"` line.
// With echo=true it echoes each input line and prints a "$ " prompt, like a tty.
func ServeFakeShell(r io.Reader, w io.Writer, echo bool, handler func(cmd string) (string, int)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	var pending []string // command lines since the last marker line
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if echo {
			fmt.Fprintf(w, "%s\r\n", line)
		}
		m := markerLine.FindStringSubmatch(line)
		if m == nil {
			pending = append(pending, line)
			continue
		}
		out, code := handler(strings.Join(pending, "\n"))
		pending = nil
		if out != "" {
			fmt.Fprintf(w, "%s\r\n", strings.ReplaceAll(out, "\n", "\r\n"))
		}
		fmt.Fprintf(w, "%s%d__\r\n", m[1], code)
		if echo {
			io.WriteString(w, "$ ")
		}
	}
}
