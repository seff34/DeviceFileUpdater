package report

import (
	"fmt"
	"io"

	"devupdater/internal/model"
)

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
		fmt.Fprintf(w, "FAILED %s", d.Host)
		if d.Error != "" {
			fmt.Fprintf(w, ": %s", d.Error)
		}
		fmt.Fprintln(w)
		for _, f := range d.Files {
			if f.Status == model.Failed && d.Error == "" {
				fmt.Fprintf(w, "  %s: %s\n", f.Remote, f.Error)
			}
		}
		if d.Post != nil && (d.Post.ExitCode != 0 || d.Post.Error != "") {
			fmt.Fprintf(w, "  post command exit %d %s\n", d.Post.ExitCode, d.Post.Error)
		}
	}
}
