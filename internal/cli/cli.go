// Package cli implements the devupdater command line.
package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"devupdater/internal/model"
	"devupdater/internal/report"
	"devupdater/internal/runner"
	"devupdater/internal/workspace"
)

const usage = `usage:
  devupdater run [-workspace DIR] [-dry-run] [-parallel N] [-only-failed REPORT_ID]
`

// runJob is a seam so tests can substitute the runner.
var runJob = runner.Run

// Main runs the CLI and returns the process exit code:
// 0 all devices ok, 1 some device failed, 2 usage or configuration error.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	return runCmd(args[1:], stdout, stderr)
}

// checkReportID mirrors report.Save's safety rules so a user-supplied ID can
// never escape the reports directory.
func checkReportID(id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") || filepath.Base(id) != id {
		return fmt.Errorf("unsafe report id %q", id)
	}
	return nil
}

func runCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("workspace", ".", "workspace folder")
	dryRun := fs.Bool("dry-run", false, "compare only, write nothing")
	parallel := fs.Int("parallel", 0, "devices at once (0 = settings.json)")
	onlyFailed := fs.String("only-failed", "", "report ID whose failed devices to retry")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: unexpected argument %q\n%s", fs.Arg(0), usage)
		return 2
	}
	ws := workspace.Workspace{Dir: *dir}
	cfgErr := func(err error) int { fmt.Fprintln(stderr, "error:", err); return 2 }

	if *parallel < 0 {
		return cfgErr(errors.New("-parallel must be >= 0"))
	}
	if *onlyFailed != "" {
		if err := checkReportID(*onlyFailed); err != nil {
			return cfgErr(err)
		}
	}
	settings, err := workspace.LoadSettings(ws.SettingsPath())
	if err != nil {
		return cfgErr(err)
	}
	if *parallel > 0 {
		settings.Parallel = *parallel
		if err := settings.Validate(); err != nil {
			return cfgErr(err)
		}
	}
	devices, err := workspace.LoadDevices(ws.DevicesPath())
	if err != nil {
		return cfgErr(err)
	}
	entries, err := workspace.LoadManifest(ws.ManifestPath())
	if err != nil {
		return cfgErr(err)
	}
	files, err := workspace.ReadFiles(ws.Dir, entries)
	if err != nil {
		return cfgErr(err)
	}
	if *onlyFailed != "" {
		prev, err := report.Load(filepath.Join(ws.ReportsDir(), *onlyFailed+".json"))
		if err != nil {
			return cfgErr(fmt.Errorf("report %s: %w", *onlyFailed, err))
		}
		devices = filterHosts(devices, report.FailedHosts(prev))
		if len(devices) == 0 {
			fmt.Fprintln(stdout, "no failed devices in", *onlyFailed)
			return 0
		}
	}
	if len(devices) == 0 {
		return cfgErr(errors.New("no devices to process"))
	}

	// Ctrl-C / SIGTERM cancel the run; runner marks pending devices cancelled
	// and we still write the report.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := 0
	res := runJob(ctx, runner.Job{
		Devices: devices, Files: files, Settings: settings,
		DryRun: *dryRun, KnownHostsPath: ws.KnownHostsPath(),
	}, func(e runner.Event) {
		if e.Type == "device_state" && (e.Stage == "done" || e.Stage == "failed") {
			done++
			fmt.Fprintf(stdout, "[%d/%d] %s %s\n", done, len(devices), e.Host, e.Stage)
		}
	})

	htmlPath, saveErr := report.Save(ws.ReportsDir(), res)
	if saveErr != nil {
		fmt.Fprintln(stderr, "error: writing report:", saveErr)
	}
	report.PrintConsole(stdout, res)
	if htmlPath != "" {
		fmt.Fprintln(stdout, "report:", htmlPath)
	}
	code := exitCode(res)
	if code == 0 && saveErr != nil {
		code = 1
	}
	return code
}

func filterHosts(ds []workspace.Device, hosts []string) []workspace.Device {
	keep := map[string]bool{}
	for _, h := range hosts {
		keep[h] = true
	}
	var out []workspace.Device
	for _, d := range ds {
		if keep[d.Host] {
			out = append(out, d)
		}
	}
	return out
}

func exitCode(r model.RunResult) int {
	for _, d := range r.Devices {
		if d.Failed() {
			return 1
		}
	}
	return 0
}
