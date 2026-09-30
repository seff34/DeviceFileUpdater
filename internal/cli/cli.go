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
	"devupdater/internal/web"
	"devupdater/internal/workspace"
)

const usage = `usage:
  devupdater ui  [-workspace DIR] [-port N] [-no-browser]
  devupdater run [-workspace DIR] [-dry-run] [-parallel N] [-only-failed REPORT_ID]

Argümansız çalıştırma arayüzü (ui) açar.
`

// runJob is a seam so tests can substitute the runner.
var runJob = runner.Run

// Main runs the CLI and returns the process exit code:
// 0 all devices ok, 1 some device failed, 2 usage or configuration error.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "-h" || args[0] == "-help" || args[0] == "--help") {
		fmt.Fprint(stdout, usage)
		return 0
	}
	if len(args) == 0 {
		return uiCmd(nil, stdout, stderr)
	}
	switch args[0] {
	case "ui":
		return uiCmd(args[1:], stdout, stderr)
	case "run":
		return runCmd(args[1:], stdout, stderr)
	}
	fmt.Fprint(stderr, usage)
	return 2
}

var serveUI = func(ctx context.Context, srv *web.Server) error { return srv.Serve(ctx) }

func uiCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dir := fs.String("workspace", "", "workspace folder to open at start")
	port := fs.Int("port", 0, "port on 127.0.0.1 (0 = random)")
	noBrowser := fs.Bool("no-browser", false, "do not open a browser")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: unexpected argument %q\n%s", fs.Arg(0), usage)
		return 2
	}
	if *port < 0 || *port > 65535 {
		fmt.Fprintln(stderr, "error: -port must be 0..65535")
		return 2
	}
	ws := *dir
	if ws != "" {
		abs, err := filepath.Abs(ws)
		if err != nil {
			fmt.Fprintln(stderr, "error:", err)
			return 2
		}
		ws = abs
	}
	srv, err := web.New(web.Options{Workspace: ws, Addr: fmt.Sprintf("127.0.0.1:%d", *port)})
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	url, err := srv.Listen()
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}
	fmt.Fprintln(stdout, "DeviceFileUpdater arayüzü:", url)
	fmt.Fprintln(stdout, "Kapatmak için Ctrl+C.")
	if !*noBrowser {
		if err := openBrowser(url); err != nil {
			fmt.Fprintln(stderr, "Tarayıcı açılamadı, adresi elle açın:", err)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := serveUI(ctx, srv); err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 1
	}
	return 0
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
		if errors.Is(err, flag.ErrHelp) {
			return 0 // flag already printed the options
		}
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
