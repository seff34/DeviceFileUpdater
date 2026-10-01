# DeviceFileUpdater — Plan 1: Engine + CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Go engine that syncs a manifest of local files to embedded Linux devices over SSH or Telnet (with or without SCP/SFTP/FTP), plus a `devupdater run` CLI that produces HTML/JSON reports.

**Architecture:** `transport` gives a uniform `Session.Exec` over SSH exec, SSH PTY shell or Telnet (marker protocol). `probe` detects device tools; `upload` provides an ordered fallback chain (sftp → scp → ftp → shell-base64 → shell-printf); `syncer` decides CREATE/UPDATE/UNCHANGED by hash and writes atomically; `runner` runs devices in a worker pool and emits events; `report` writes HTML/JSON/console output. Plan 2 (web UI) builds on `workspace`, `runner` and `report` without changing them.

**Tech Stack:** Go 1.25+ (CGO off), `golang.org/x/crypto/ssh`, `github.com/pkg/sftp`, `github.com/jlaffaye/ftp`, stdlib `html/template`. Integration tests: Docker Compose (Alpine + busybox devices).

**Spec:** `docs/superpowers/specs/2026-09-30-device-file-updater-design.md`

## Global Constraints

- Single static binary, `CGO_ENABLED=0`; targets windows/linux/darwin × amd64/arm64.
- Module path: `devupdater`. All packages under `internal/`.
- Passwords never appear in reports, JSON, logs, errors or console output.
- Statuses exactly: `UNCHANGED`, `CREATED`, `UPDATED`, `WOULD_CREATE`, `WOULD_UPDATE`, `FAILED`.
- Protocol order per device: SSH first, then Telnet; same credentials; one retry on connection failure.
- Upload order: `sftp` → `scp` → `ftp` → `shell-base64` → `shell-printf`. Hash order: `sha256sum` → `md5sum` → readback (`base64` → `od` → `hexdump`) → none.
- Temp file: `<remote>.devupd.tmp`; backup: `<remote>.bak` (overwritten).
- New file default mode `0644`; empty manifest mode on update keeps existing mode.
- Shell command lines sent over marker sessions stay ≤ 1024 bytes.
- CLI exit codes: `0` no FAILED, `1` at least one FAILED, `2` config/validation error.
- Settings defaults: `parallel 10`, `backup true`, `post_command ""`, `post_command_policy "on_change"`, `connect_timeout_sec 10`, `command_timeout_sec 30`, `strict_host_key false`.
- Unit tests that execute a real `/bin/sh` carry `//go:build !windows`.

## File Structure

```
go.mod
cmd/devupdater/main.go            entry; delegates to internal/cli
internal/model/model.go           Status, LocalFile, FileResult, DeviceResult, RunResult
internal/shell/quote.go           POSIX single-quote quoting
internal/shell/marker.go          MarkerSession: Exec/Expect/Send over any byte stream (end markers)
internal/testutil/fakeshell.go    fake marker-protocol shell for tests
internal/testutil/sessions.go     FakeSession (scripted)
internal/testutil/localshell.go   LocalShell (runs real sh, !windows)
internal/testutil/telnetserver.go fake telnet server
internal/testutil/sshserver.go    fake ssh server (exec, pty, sftp)
internal/workspace/devices.go     devices.csv load/save/validate
internal/workspace/manifest.go    manifest.csv load/save/validate + ReadFiles
internal/workspace/settings.go    settings.json load/save/defaults
internal/workspace/workspace.go   path helpers
internal/transport/session.go     Session interface, Result, Options
internal/transport/telnet.go      telnet client (IAC, login) → MarkerSession
internal/transport/ssh.go         SSH exec session + PTY fallback + host keys
internal/transport/hostkeys.go    TOFU known_hosts
internal/transport/dial.go        Dial(): SSH→Telnet, retry
internal/probe/probe.go           Caps detection
internal/upload/upload.go         Uploader interface, Chain, Select
internal/upload/shell.go          shell-base64, shell-printf uploaders
internal/upload/sftp.go           sftp uploader
internal/upload/scp.go            scp uploader
internal/upload/ftp.go            ftp uploader
internal/syncer/hasher.go         remote hashing strategies
internal/syncer/remote.go         exists / mode helpers
internal/syncer/syncer.go         SyncFile decision + atomic write
internal/runner/runner.go         worker pool, events, post_command
internal/report/report.go         JSON save/load, Summary, FailedHosts
internal/report/html.go           HTML report (embedded template)
internal/report/console.go        console summary
internal/cli/cli.go               `run` subcommand, exit codes
test/integration/...              docker compose device matrix + tests
Makefile                          test, integration, release
```

---
### Task 1: Module scaffold, shared model, shell quoting

**Files:**
- Create: `go.mod`, `.gitignore`, `internal/model/model.go`, `internal/shell/quote.go`
- Test: `internal/shell/quote_test.go`, `internal/model/model_test.go`

**Interfaces:**
- Produces:
  - `model.Status` consts `Unchanged, Created, Updated, WouldCreate, WouldUpdate, Failed`
  - `model.LocalFile{Local, Remote, Mode string; Data []byte; SHA256 string}`
  - `model.FileResult{Remote string; Status Status; Method string; DurationMS int64; Error, Note string}`
  - `model.PostResult{Command string; ExitCode int; Output, Error string}`
  - `model.DeviceResult{Host, Protocol, UploadMethod, HashMethod string; Tools []string; DurationMS int64; Error string; Files []FileResult; Post *PostResult}` + `(DeviceResult).Failed() bool`
  - `model.RunResult{ID string; Started, Finished time.Time; DryRun bool; Parallel int; Files []string; Devices []DeviceResult}`
  - `shell.Quote(s string) string`

- [ ] **Step 1: Init module**

```bash
cd DeviceFileUpdater
go mod init devupdater
go mod edit -go=1.25
printf 'dist/\nweb/node_modules/\n*.test\n' > .gitignore
```

- [ ] **Step 2: Write failing tests**

`internal/shell/quote_test.go`:
```go
package shell

import "testing"

func TestQuote(t *testing.T) {
	cases := map[string]string{
		"/opt/app/x.bin": `'/opt/app/x.bin'`,
		"a b":            `'a b'`,
		"it's":           `'it'\''s'`,
		"":               `''`,
	}
	for in, want := range cases {
		if got := Quote(in); got != want {
			t.Errorf("Quote(%q) = %s, want %s", in, got, want)
		}
	}
}
```

`internal/model/model_test.go`:
```go
package model

import "testing"

func TestDeviceFailed(t *testing.T) {
	ok := DeviceResult{Files: []FileResult{{Status: Unchanged}, {Status: Created}}}
	if ok.Failed() {
		t.Fatal("expected not failed")
	}
	byFile := DeviceResult{Files: []FileResult{{Status: Failed}}}
	if !byFile.Failed() {
		t.Fatal("expected failed by file")
	}
	byErr := DeviceResult{Error: "connect: refused"}
	if !byErr.Failed() {
		t.Fatal("expected failed by device error")
	}
	byPost := DeviceResult{Post: &PostResult{ExitCode: 1}}
	if !byPost.Failed() {
		t.Fatal("expected failed by post command")
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run: `go test ./internal/...`
Expected: FAIL — `undefined: Quote`, `undefined: DeviceResult`.

- [ ] **Step 4: Implement**

`internal/shell/quote.go`:
```go
// Package shell holds helpers for talking to POSIX shells on devices.
package shell

import "strings"

// Quote wraps s in single quotes so any POSIX sh treats it as one literal word.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

`internal/model/model.go`:
```go
// Package model holds result types shared by syncer, runner, report and web.
package model

import "time"

type Status string

const (
	Unchanged   Status = "UNCHANGED"
	Created     Status = "CREATED"
	Updated     Status = "UPDATED"
	WouldCreate Status = "WOULD_CREATE"
	WouldUpdate Status = "WOULD_UPDATE"
	Failed      Status = "FAILED"
)

// LocalFile is a manifest entry with its content loaded from the host.
type LocalFile struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
	Mode   string `json:"mode"`
	Data   []byte `json:"-"`
	SHA256 string `json:"sha256"`
}

type FileResult struct {
	Remote     string `json:"remote"`
	Status     Status `json:"status"`
	Method     string `json:"method,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Note       string `json:"note,omitempty"`
}

type PostResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

type DeviceResult struct {
	Host         string       `json:"host"`
	Protocol     string       `json:"protocol,omitempty"`
	UploadMethod string       `json:"upload_method,omitempty"`
	HashMethod   string       `json:"hash_method,omitempty"`
	Tools        []string     `json:"tools,omitempty"`
	DurationMS   int64        `json:"duration_ms"`
	Error        string       `json:"error,omitempty"`
	Files        []FileResult `json:"files"`
	Post         *PostResult  `json:"post,omitempty"`
}

// Failed reports whether anything on this device went wrong.
func (d DeviceResult) Failed() bool {
	if d.Error != "" {
		return true
	}
	if d.Post != nil && (d.Post.ExitCode != 0 || d.Post.Error != "") {
		return true
	}
	for _, f := range d.Files {
		if f.Status == Failed {
			return true
		}
	}
	return false
}

type RunResult struct {
	ID       string         `json:"id"`
	Started  time.Time      `json:"started"`
	Finished time.Time      `json:"finished"`
	DryRun   bool           `json:"dry_run"`
	Parallel int            `json:"parallel"`
	Files    []string       `json:"files"`
	Devices  []DeviceResult `json:"devices"`
}
```

- [ ] **Step 5: Run, verify pass**

Run: `go test ./internal/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod .gitignore internal/model internal/shell
git commit -m "feat: scaffold module, shared result model, shell quoting"
```

---

### Task 2: Workspace files (devices, manifest, settings)

**Files:**
- Create: `internal/workspace/workspace.go`, `internal/workspace/devices.go`, `internal/workspace/manifest.go`, `internal/workspace/settings.go`
- Test: `internal/workspace/devices_test.go`, `internal/workspace/manifest_test.go`, `internal/workspace/settings_test.go`

**Interfaces:**
- Consumes: `model.LocalFile`
- Produces:
  - `workspace.Workspace{Dir string}` with `DevicesPath()`, `ManifestPath()`, `SettingsPath()`, `FilesDir()`, `ReportsDir()`, `KnownHostsPath() string`
  - `workspace.Device{Host, Username, Password string}`; `(Device).Addr(defaultPort int) string`; `(Device).HostOnly() string`
  - `workspace.LoadDevices(path string) ([]Device, error)`, `workspace.SaveDevices(path string, ds []Device) error`, `workspace.ParseDevices(r io.Reader) ([]Device, error)`
  - `workspace.Entry{LocalPath, RemotePath, Mode string}`
  - `workspace.LoadManifest(path string) ([]Entry, error)`, `workspace.SaveManifest(path string, es []Entry) error`, `workspace.ValidateEntries(es []Entry) error`
  - `workspace.ReadFiles(baseDir string, es []Entry) ([]model.LocalFile, error)`
  - `workspace.Settings{Parallel int; Backup bool; PostCommand, PostCommandPolicy string; ConnectTimeoutSec, CommandTimeoutSec int; StrictHostKey bool}` (JSON tags as spec), `workspace.DefaultSettings()`, `LoadSettings(path) (Settings, error)`, `SaveSettings(path, s) error`, `(Settings).Validate() error`

- [ ] **Step 1: Write failing tests**

`internal/workspace/devices_test.go`:
```go
package workspace

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseDevices(t *testing.T) {
	in := "ip,username,password\n192.168.1.10,root,s1\n192.168.1.11:2222,admin,\"s,2\"\n"
	ds, err := ParseDevices(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(ds) != 2 || ds[1].Password != "s,2" || ds[1].Host != "192.168.1.11:2222" {
		t.Fatalf("got %+v", ds)
	}
	if ds[0].Addr(22) != "192.168.1.10:22" || ds[1].Addr(22) != "192.168.1.11:2222" {
		t.Fatalf("addr: %s %s", ds[0].Addr(22), ds[1].Addr(22))
	}
	if ds[1].HostOnly() != "192.168.1.11" {
		t.Fatalf("hostonly %s", ds[1].HostOnly())
	}
}

func TestParseDevicesErrors(t *testing.T) {
	bad := []string{
		"",
		"host,user,pass\n1.1.1.1,a,b\n",               // wrong header
		"ip,username,password\n1.1.1.1,a,b\n1.1.1.1,c,d\n", // duplicate
		"ip,username,password\n,a,b\n",                // empty ip
		"ip,username,password\n1.1.1.1,,b\n",          // empty user
	}
	for _, in := range bad {
		if _, err := ParseDevices(strings.NewReader(in)); err == nil {
			t.Errorf("expected error for %q", in)
		}
	}
}

func TestSaveLoadDevicesRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "devices.csv")
	want := []Device{{Host: "10.0.0.1", Username: "root", Password: `p"w,1`}}
	if err := SaveDevices(p, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadDevices(p)
	if err != nil || len(got) != 1 || got[0] != want[0] {
		t.Fatalf("got %+v err %v", got, err)
	}
}
```

`internal/workspace/manifest_test.go`:
```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRoundTripAndValidate(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.csv")
	es := []Entry{
		{LocalPath: "files/a.bin", RemotePath: "/opt/a.bin", Mode: "0755"},
		{LocalPath: "files/b.conf", RemotePath: "/etc/b.conf", Mode: ""},
	}
	if err := SaveManifest(p, es); err != nil {
		t.Fatal(err)
	}
	got, err := LoadManifest(p)
	if err != nil || len(got) != 2 || got[0] != es[0] || got[1] != es[1] {
		t.Fatalf("got %+v err %v", got, err)
	}
}

func TestValidateEntries(t *testing.T) {
	bad := [][]Entry{
		{{LocalPath: "a", RemotePath: "rel/path"}},
		{{LocalPath: "a", RemotePath: "/x", Mode: "999"}},
		{{LocalPath: "", RemotePath: "/x"}},
		{{LocalPath: "a", RemotePath: "/x"}, {LocalPath: "b", RemotePath: "/x"}},
		{},
	}
	for i, es := range bad {
		if err := ValidateEntries(es); err == nil {
			t.Errorf("case %d: expected error", i)
		}
	}
	if err := ValidateEntries([]Entry{{LocalPath: "a", RemotePath: "/x", Mode: "644"}}); err != nil {
		t.Fatal(err)
	}
}

func TestReadFiles(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "files"), 0o755)
	os.WriteFile(filepath.Join(dir, "files", "a.txt"), []byte("hello"), 0o644)
	fs, err := ReadFiles(dir, []Entry{{LocalPath: "files/a.txt", RemotePath: "/tmp/a.txt"}})
	if err != nil {
		t.Fatal(err)
	}
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if fs[0].SHA256 != want || string(fs[0].Data) != "hello" || fs[0].Remote != "/tmp/a.txt" {
		t.Fatalf("got %+v", fs[0])
	}
	if _, err := ReadFiles(dir, []Entry{{LocalPath: "missing", RemotePath: "/x"}}); err == nil {
		t.Fatal("expected missing file error")
	}
}
```

`internal/workspace/settings_test.go`:
```go
package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsDefaultsWhenMissing(t *testing.T) {
	s, err := LoadSettings(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil || s != DefaultSettings() {
		t.Fatalf("got %+v err %v", s, err)
	}
}

func TestSettingsPartialFileKeepsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(p, []byte(`{"parallel": 3}`), 0o644)
	s, err := LoadSettings(p)
	if err != nil || s.Parallel != 3 || !s.Backup || s.CommandTimeoutSec != 30 {
		t.Fatalf("got %+v err %v", s, err)
	}
}

func TestSettingsValidate(t *testing.T) {
	s := DefaultSettings()
	s.PostCommandPolicy = "sometimes"
	if s.Validate() == nil {
		t.Fatal("expected policy error")
	}
	s = DefaultSettings()
	s.Parallel = 0
	if s.Validate() == nil {
		t.Fatal("expected parallel error")
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/workspace/`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement**

`internal/workspace/workspace.go`:
```go
// Package workspace reads and writes the operator's working folder.
package workspace

import "path/filepath"

type Workspace struct{ Dir string }

func (w Workspace) DevicesPath() string    { return filepath.Join(w.Dir, "devices.csv") }
func (w Workspace) ManifestPath() string   { return filepath.Join(w.Dir, "manifest.csv") }
func (w Workspace) SettingsPath() string   { return filepath.Join(w.Dir, "settings.json") }
func (w Workspace) FilesDir() string       { return filepath.Join(w.Dir, "files") }
func (w Workspace) ReportsDir() string     { return filepath.Join(w.Dir, "reports") }
func (w Workspace) KnownHostsPath() string { return filepath.Join(w.Dir, "known_hosts") }
```

`internal/workspace/devices.go`:
```go
package workspace

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
)

type Device struct {
	Host     string `json:"host"` // ip, hostname, or host:port
	Username string `json:"username"`
	Password string `json:"password"`
}

// Addr returns host:port, using defaultPort when the CSV gave no port.
func (d Device) Addr(defaultPort int) string {
	if _, _, err := net.SplitHostPort(d.Host); err == nil {
		return d.Host
	}
	return net.JoinHostPort(d.Host, strconv.Itoa(defaultPort))
}

// HostOnly strips an optional port.
func (d Device) HostOnly() string {
	if h, _, err := net.SplitHostPort(d.Host); err == nil {
		return h
	}
	return d.Host
}

var deviceHeader = []string{"ip", "username", "password"}

func ParseDevices(r io.Reader) ([]Device, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("devices.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("devices.csv: empty file")
	}
	for i, h := range deviceHeader {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != h {
			return nil, fmt.Errorf("devices.csv: header must be %s", strings.Join(deviceHeader, ","))
		}
	}
	seen := map[string]bool{}
	var out []Device
	for i, row := range rows[1:] {
		d := Device{Host: strings.TrimSpace(row[0]), Username: strings.TrimSpace(row[1]), Password: row[2]}
		line := i + 2
		if d.Host == "" || d.Username == "" {
			return nil, fmt.Errorf("devices.csv line %d: ip and username are required", line)
		}
		if seen[d.Host] {
			return nil, fmt.Errorf("devices.csv line %d: duplicate ip %s", line, d.Host)
		}
		seen[d.Host] = true
		out = append(out, d)
	}
	return out, nil
}

func LoadDevices(path string) ([]Device, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseDevices(f)
}

func SaveDevices(path string, ds []Device) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write(deviceHeader)
	for _, d := range ds {
		w.Write([]string{d.Host, d.Username, d.Password})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
```

`internal/workspace/manifest.go`:
```go
package workspace

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"devupdater/internal/model"
)

type Entry struct {
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
	Mode       string `json:"mode"`
}

var manifestHeader = []string{"local_path", "remote_path", "mode"}
var modeRe = regexp.MustCompile(`^[0-7]{3,4}$`)

func LoadManifest(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cr.FieldsPerRecord = 3
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("manifest.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("manifest.csv: empty file")
	}
	for i, h := range manifestHeader {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != h {
			return nil, fmt.Errorf("manifest.csv: header must be %s", strings.Join(manifestHeader, ","))
		}
	}
	var es []Entry
	for _, r := range rows[1:] {
		es = append(es, Entry{
			LocalPath:  strings.TrimSpace(r[0]),
			RemotePath: strings.TrimSpace(r[1]),
			Mode:       strings.TrimSpace(r[2]),
		})
	}
	return es, ValidateEntries(es)
}

func SaveManifest(path string, es []Entry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write(manifestHeader)
	for _, e := range es {
		w.Write([]string{e.LocalPath, e.RemotePath, e.Mode})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func ValidateEntries(es []Entry) error {
	if len(es) == 0 {
		return fmt.Errorf("manifest: no files")
	}
	seen := map[string]bool{}
	for i, e := range es {
		n := i + 1
		if e.LocalPath == "" {
			return fmt.Errorf("manifest row %d: local_path is required", n)
		}
		if !strings.HasPrefix(e.RemotePath, "/") {
			return fmt.Errorf("manifest row %d: remote_path must be absolute: %q", n, e.RemotePath)
		}
		if e.Mode != "" && !modeRe.MatchString(e.Mode) {
			return fmt.Errorf("manifest row %d: mode must be octal like 0644: %q", n, e.Mode)
		}
		if seen[e.RemotePath] {
			return fmt.Errorf("manifest row %d: duplicate remote_path %s", n, e.RemotePath)
		}
		seen[e.RemotePath] = true
	}
	return nil
}

// ReadFiles loads each entry's content; relative local paths resolve against baseDir.
func ReadFiles(baseDir string, es []Entry) ([]model.LocalFile, error) {
	var out []model.LocalFile
	for _, e := range es {
		p := e.LocalPath
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
		sum := sha256.Sum256(data)
		out = append(out, model.LocalFile{
			Local: p, Remote: e.RemotePath, Mode: e.Mode,
			Data: data, SHA256: hex.EncodeToString(sum[:]),
		})
	}
	return out, nil
}
```

`internal/workspace/settings.go`:
```go
package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type Settings struct {
	Parallel          int    `json:"parallel"`
	Backup            bool   `json:"backup"`
	PostCommand       string `json:"post_command"`
	PostCommandPolicy string `json:"post_command_policy"`
	ConnectTimeoutSec int    `json:"connect_timeout_sec"`
	CommandTimeoutSec int    `json:"command_timeout_sec"`
	StrictHostKey     bool   `json:"strict_host_key"`
}

func DefaultSettings() Settings {
	return Settings{
		Parallel: 10, Backup: true, PostCommandPolicy: "on_change",
		ConnectTimeoutSec: 10, CommandTimeoutSec: 30,
	}
}

// LoadSettings returns defaults for a missing file; fields absent from the file keep defaults.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("settings.json: %w", err)
	}
	return s, s.Validate()
}

func SaveSettings(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

func (s Settings) Validate() error {
	if s.Parallel < 1 || s.Parallel > 200 {
		return fmt.Errorf("settings: parallel must be 1..200")
	}
	switch s.PostCommandPolicy {
	case "on_change", "always", "never":
	default:
		return fmt.Errorf("settings: post_command_policy must be on_change, always or never")
	}
	if s.ConnectTimeoutSec < 1 || s.CommandTimeoutSec < 1 {
		return fmt.Errorf("settings: timeouts must be >= 1 second")
	}
	return nil
}
```

- [ ] **Step 4: Run, verify pass**

Run: `go test ./internal/workspace/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/workspace
git commit -m "feat: workspace devices/manifest/settings files"
```

---

### Task 3: Marker session (command execution over a raw shell stream)

Telnet and exec-less SSH only give a byte stream to an interactive shell. `MarkerSession` turns that into `Exec(cmd) → (output, exitCode)` by appending `; echo "__DU_<nonce>_<n>_$?__"` to each command and reading until the expanded marker appears. The echoed command line contains the literal `$?`, so it never matches the marker regex; if the terminal echoes, everything up to the echoed line is stripped.

**Files:**
- Create: `internal/shell/marker.go`, `internal/testutil/fakeshell.go`
- Test: `internal/shell/marker_test.go`

**Interfaces:**
- Produces:
  - `shell.NewMarkerSession(r io.Reader, w io.Writer, newline string) *MarkerSession` (starts a reader goroutine)
  - `(*MarkerSession).Send(s string) error` — writes raw text
  - `(*MarkerSession).Expect(ctx context.Context, re *regexp.Regexp) (string, error)` — reads until `re` matches buffered output; returns and consumes text up to match end
  - `(*MarkerSession).Init(ctx context.Context) error` — `PS1=''; PS2=''; stty -echo 2>/dev/null; true`
  - `(*MarkerSession).Exec(ctx context.Context, cmd string) (string, int, error)`
  - `testutil.ServeFakeShell(r io.Reader, w io.Writer, echo bool, handler func(cmd string) (string, int))`

- [ ] **Step 1: Write fake shell helper**

`internal/testutil/fakeshell.go`:
```go
// Package testutil holds fakes shared by tests across packages.
package testutil

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var markerLine = regexp.MustCompile(`^(.*); echo "(__DU_[0-9a-f]+_\d+_)\$\?__"$`)

// ServeFakeShell imitates an interactive sh that speaks the marker protocol.
// With echo=true it echoes each input line and prints a "$ " prompt, like a tty.
func ServeFakeShell(r io.Reader, w io.Writer, echo bool, handler func(cmd string) (string, int)) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		if echo {
			fmt.Fprintf(w, "%s\r\n", line)
		}
		m := markerLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out, code := handler(m[1])
		if out != "" {
			fmt.Fprintf(w, "%s\r\n", strings.ReplaceAll(out, "\n", "\r\n"))
		}
		fmt.Fprintf(w, "%s%d__\r\n", m[2], code)
		if echo {
			io.WriteString(w, "$ ")
		}
	}
}
```

- [ ] **Step 2: Write failing tests**

`internal/shell/marker_test.go`:
```go
package shell

import (
	"context"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

// pipeShell connects a MarkerSession to a fake shell and returns it.
func pipeShell(t *testing.T, echo bool, h func(string) (string, int)) *MarkerSession {
	t.Helper()
	cr, sw := io.Pipe() // shell -> client
	sr, cw := io.Pipe() // client -> shell
	go testutil.ServeFakeShell(sr, sw, echo, h)
	t.Cleanup(func() { cw.Close(); sw.Close() })
	return NewMarkerSession(cr, cw, "\n")
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestExecOutputAndCode(t *testing.T) {
	for _, echo := range []bool{false, true} {
		m := pipeShell(t, echo, func(cmd string) (string, int) {
			if cmd == "false" {
				return "", 1
			}
			return "line1\nline2", 0
		})
		out, code, err := m.Exec(ctx(t), "cat x")
		if err != nil || code != 0 || out != "line1\nline2" {
			t.Fatalf("echo=%v: out=%q code=%d err=%v", echo, out, code, err)
		}
		out, code, err = m.Exec(ctx(t), "false")
		if err != nil || code != 1 || out != "" {
			t.Fatalf("echo=%v second: out=%q code=%d err=%v", echo, out, code, err)
		}
	}
}

func TestExecTimeoutBreaksSession(t *testing.T) {
	block := make(chan struct{})
	m := pipeShell(t, false, func(string) (string, int) { <-block; return "", 0 })
	defer close(block)
	c, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, _, err := m.Exec(c, "sleep"); err == nil {
		t.Fatal("expected timeout")
	}
	if _, _, err := m.Exec(ctx(t), "echo"); err == nil || !strings.Contains(err.Error(), "out of sync") {
		t.Fatalf("expected out of sync, got %v", err)
	}
}

func TestExpect(t *testing.T) {
	cr, sw := io.Pipe()
	m := NewMarkerSession(cr, io.Discard, "\n")
	go io.WriteString(sw, "Welcome\r\nbox login: ")
	got, err := m.Expect(ctx(t), regexp.MustCompile(`login: $`))
	if err != nil || !strings.HasSuffix(got, "login: ") {
		t.Fatalf("got %q err %v", got, err)
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run: `go test ./internal/shell/`
Expected: FAIL — `undefined: NewMarkerSession`.

- [ ] **Step 4: Implement**

`internal/shell/marker.go`:
```go
package shell

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"sync"
)

// MarkerSession runs commands on an interactive shell reachable only as a byte stream.
type MarkerSession struct {
	mu      sync.Mutex
	w       io.Writer
	newline string
	nonce   string
	n       int
	in      chan []byte
	readErr error
	buf     []byte
	broken  error
}

func NewMarkerSession(r io.Reader, w io.Writer, newline string) *MarkerSession {
	nb := make([]byte, 4)
	rand.Read(nb)
	m := &MarkerSession{w: w, newline: newline, nonce: hex.EncodeToString(nb), in: make(chan []byte, 64)}
	go m.pump(r)
	return m
}

func (m *MarkerSession) pump(r io.Reader) {
	b := make([]byte, 4096)
	for {
		n, err := r.Read(b)
		if n > 0 {
			c := make([]byte, n)
			copy(c, b[:n])
			m.in <- c
		}
		if err != nil {
			m.readErr = err
			close(m.in)
			return
		}
	}
}

// readMore appends the next chunk (CR and NUL removed) or fails on ctx/EOF.
func (m *MarkerSession) readMore(ctx context.Context) error {
	select {
	case b, ok := <-m.in:
		if !ok {
			if m.readErr != nil {
				return fmt.Errorf("connection closed: %w", m.readErr)
			}
			return io.EOF
		}
		b = bytes.ReplaceAll(b, []byte{'\r'}, nil)
		b = bytes.ReplaceAll(b, []byte{0}, nil)
		m.buf = append(m.buf, b...)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MarkerSession) Send(s string) error {
	_, err := io.WriteString(m.w, s)
	return err
}

func (m *MarkerSession) Expect(ctx context.Context, re *regexp.Regexp) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for {
		if loc := re.FindIndex(m.buf); loc != nil {
			out := string(m.buf[:loc[1]])
			m.buf = m.buf[loc[1]:]
			return out, nil
		}
		if err := m.readMore(ctx); err != nil {
			return string(m.buf), err
		}
	}
}

// Init clears prompts and disables echo so later output is clean.
func (m *MarkerSession) Init(ctx context.Context) error {
	_, _, err := m.Exec(ctx, "PS1=''; PS2=''; stty -echo 2>/dev/null; true")
	return err
}

func (m *MarkerSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.broken != nil {
		return "", -1, fmt.Errorf("session out of sync after earlier error: %w", m.broken)
	}
	m.n++
	tag := fmt.Sprintf("__DU_%s_%d_", m.nonce, m.n)
	echoed := []byte(tag + "$?__\"")
	re := regexp.MustCompile(regexp.QuoteMeta(tag) + `(\d+)__`)
	if _, err := io.WriteString(m.w, cmd+`; echo "`+tag+`$?__"`+m.newline); err != nil {
		m.broken = err
		return "", -1, err
	}
	for {
		if loc := re.FindSubmatchIndex(m.buf); loc != nil {
			out := m.buf[:loc[0]]
			code, _ := strconv.Atoi(string(m.buf[loc[2]:loc[3]]))
			rest := m.buf[loc[1]:]
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
			m.buf = append([]byte(nil), rest...)
			// Drop the echoed command line (and any prompt before it) if the tty echoed.
			if i := bytes.LastIndex(out, echoed); i >= 0 {
				out = out[i+len(echoed):]
				if j := bytes.IndexByte(out, '\n'); j >= 0 {
					out = out[j+1:]
				} else {
					out = nil
				}
			}
			return string(bytes.TrimSuffix(out, []byte{'\n'})), code, nil
		}
		if err := m.readMore(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				m.broken = err
			}
			return string(m.buf), -1, err
		}
	}
}
```

- [ ] **Step 5: Run, verify pass**

Run: `go test -race ./internal/shell/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/shell internal/testutil
git commit -m "feat: marker-protocol shell session for telnet and PTY shells"
```

---

### Task 4: Session interface + Telnet transport

**Files:**
- Create: `internal/transport/session.go`, `internal/transport/telnet.go`, `internal/testutil/telnetserver.go`
- Test: `internal/transport/telnet_test.go`

**Interfaces:**
- Consumes: `shell.NewMarkerSession`, `(*MarkerSession).Send/Expect/Init/Exec`, `testutil.ServeFakeShell`
- Produces:
  - `transport.Session` interface: `Exec(ctx, cmd string) (string, int, error)`, `Protocol() string`, `Close() error`
  - `transport.Options{ConnectTimeout, CommandTimeout time.Duration; StrictHostKey bool; KnownHostsPath string}`
  - `transport.ErrAuth` (sentinel; wrap with `%w` for credential rejections)
  - `transport.DialTelnet(ctx, addr, user, pass string, opt Options) (Session, error)`
  - `testutil.StartFakeTelnet(t *testing.T, user, pass string, handler func(cmd string) (string, int)) string` (returns `127.0.0.1:port`)

- [ ] **Step 1: Write fake telnet server**

`internal/testutil/telnetserver.go`:
```go
package testutil

import (
	"bufio"
	"io"
	"net"
	"strings"
	"testing"
)

// StartFakeTelnet serves a busybox-like telnet login followed by a fake shell.
func StartFakeTelnet(t *testing.T, user, pass string, handler func(cmd string) (string, int)) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go serveTelnet(c, user, pass, handler)
		}
	}()
	return ln.Addr().String()
}

func serveTelnet(c net.Conn, user, pass string, handler func(string) (string, int)) {
	defer c.Close()
	c.Write([]byte{255, 253, 1}) // IAC DO ECHO — client must filter and answer
	r := bufio.NewReader(c)
	readLine := func() (string, error) {
		s, err := r.ReadString('\n')
		// drop any IAC replies (3-byte sequences) that precede text
		for strings.HasPrefix(s, "\xff") && len(s) >= 3 {
			s = s[3:]
		}
		return strings.TrimRight(s, "\r\n"), err
	}
	for {
		io.WriteString(c, "Welcome\r\nbox login: ")
		u, err := readLine()
		if err != nil {
			return
		}
		io.WriteString(c, "Password: ")
		p, err := readLine()
		if err != nil {
			return
		}
		if u == user && p == pass {
			break
		}
		io.WriteString(c, "\r\nLogin incorrect\r\n")
	}
	io.WriteString(c, "\r\n~ $ ")
	ServeFakeShell(r, c, true, handler)
}
```

- [ ] **Step 2: Write failing tests**

`internal/transport/telnet_test.go`:
```go
package transport

import (
	"context"
	"errors"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func testOpts() Options {
	return Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second}
}

func TestTelnetExec(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(cmd string) (string, int) {
		if cmd == "uname" {
			return "Linux", 0
		}
		return "", 0
	})
	s, err := DialTelnet(context.Background(), addr, "dev", "pw", testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Protocol() != "telnet" {
		t.Fatalf("protocol %s", s.Protocol())
	}
	out, code, err := s.Exec(context.Background(), "uname")
	if err != nil || code != 0 || out != "Linux" {
		t.Fatalf("out=%q code=%d err=%v", out, code, err)
	}
}

func TestTelnetBadPassword(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "dev", "pw", func(string) (string, int) { return "", 0 })
	_, err := DialTelnet(context.Background(), addr, "dev", "wrong", testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}

func TestTelnetRefused(t *testing.T) {
	_, err := DialTelnet(context.Background(), "127.0.0.1:1", "a", "b", testOpts())
	if err == nil || errors.Is(err, ErrAuth) {
		t.Fatalf("expected connection error, got %v", err)
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run: `go test ./internal/transport/`
Expected: FAIL — `undefined: DialTelnet`.

- [ ] **Step 4: Implement**

`internal/transport/session.go`:
```go
// Package transport connects to devices and exposes a uniform command session.
package transport

import (
	"context"
	"errors"
	"time"
)

// Session runs shell commands on a device. Output is stdout+stderr combined.
type Session interface {
	Exec(ctx context.Context, cmd string) (string, int, error)
	Protocol() string
	Close() error
}

type Options struct {
	ConnectTimeout time.Duration
	CommandTimeout time.Duration
	StrictHostKey  bool
	KnownHostsPath string
}

// ErrAuth marks credential rejection (no point retrying the same credentials).
var ErrAuth = errors.New("authentication failed")
```

`internal/transport/telnet.go`:
```go
package transport

import (
	"context"
	"fmt"
	"net"
	"regexp"

	"devupdater/internal/shell"
)

const (
	tIAC  = 255
	tDONT = 254
	tDO   = 253
	tWONT = 252
	tWILL = 251
	tSB   = 250
	tSE   = 240

	optEcho = 1
	optSGA  = 3
)

// iacReader strips telnet negotiation from the stream and answers it.
type iacReader struct {
	c     net.Conn
	state int
	cmd   byte
}

func (r *iacReader) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))
	for {
		n, err := r.c.Read(buf)
		out := 0
		for _, b := range buf[:n] {
			switch r.state {
			case 0:
				if b == tIAC {
					r.state = 1
				} else {
					p[out] = b
					out++
				}
			case 1:
				switch b {
				case tIAC:
					p[out] = b
					out++
					r.state = 0
				case tDO, tDONT, tWILL, tWONT:
					r.cmd, r.state = b, 2
				case tSB:
					r.state = 3
				default:
					r.state = 0
				}
			case 2:
				r.answer(r.cmd, b)
				r.state = 0
			case 3:
				if b == tIAC {
					r.state = 4
				}
			case 4:
				if b == tSE {
					r.state = 0
				} else {
					r.state = 3
				}
			}
		}
		if out > 0 || err != nil {
			return out, err
		}
	}
}

// answer accepts server echo + suppress-go-ahead and refuses everything else.
func (r *iacReader) answer(cmd, opt byte) {
	switch cmd {
	case tDO:
		if opt == optSGA {
			r.c.Write([]byte{tIAC, tWILL, opt})
		} else {
			r.c.Write([]byte{tIAC, tWONT, opt})
		}
	case tWILL:
		if opt == optEcho || opt == optSGA {
			r.c.Write([]byte{tIAC, tDO, opt})
		} else {
			r.c.Write([]byte{tIAC, tDONT, opt})
		}
	}
}

var (
	reLogin     = regexp.MustCompile(`(?i)(login|username|user name)\s*:\s*$`)
	rePassOrSh  = regexp.MustCompile(`(?i)(password\s*:\s*$)|([#$>%]\s*$)`)
	reAfterPass = regexp.MustCompile(`(?i)(incorrect|failed|denied|invalid)|(login\s*:\s*$)|([#$>%]\s*$)`)
)

type telnetSession struct {
	conn net.Conn
	m    *shell.MarkerSession
}

func DialTelnet(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
	d := net.Dialer{Timeout: opt.ConnectTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("telnet connect %s: %w", addr, err)
	}
	m := shell.NewMarkerSession(&iacReader{c: conn}, conn, "\r\n")
	lctx, cancel := context.WithTimeout(ctx, opt.ConnectTimeout+opt.CommandTimeout)
	defer cancel()
	fail := func(e error) (Session, error) { conn.Close(); return nil, e }

	if _, err := m.Expect(lctx, reLogin); err != nil {
		return fail(fmt.Errorf("telnet %s: no login prompt: %w", addr, err))
	}
	m.Send(user + "\r\n")
	got, err := m.Expect(lctx, rePassOrSh)
	if err != nil {
		return fail(fmt.Errorf("telnet %s: no password prompt: %w", addr, err))
	}
	if sm := rePassOrSh.FindStringSubmatch(got); sm[1] != "" {
		m.Send(pass + "\r\n")
		got, err = m.Expect(lctx, reAfterPass)
		if err != nil {
			return fail(fmt.Errorf("telnet %s: no shell prompt after login: %w", addr, err))
		}
		if sm := reAfterPass.FindStringSubmatch(got); sm[1] != "" || sm[2] != "" {
			return fail(fmt.Errorf("telnet %s: %w", addr, ErrAuth))
		}
	}
	if err := m.Init(lctx); err != nil {
		return fail(fmt.Errorf("telnet %s: shell init: %w", addr, err))
	}
	return &telnetSession{conn: conn, m: m}, nil
}

func (s *telnetSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	return s.m.Exec(ctx, cmd)
}
func (s *telnetSession) Protocol() string { return "telnet" }
func (s *telnetSession) Close() error     { return s.conn.Close() }
```

- [ ] **Step 5: Run, verify pass**

Run: `go test -race ./internal/transport/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/transport internal/testutil
git commit -m "feat: telnet transport with IAC handling and login"
```

---

### Task 5: SSH transport (exec, PTY fallback, host keys)

**Files:**
- Create: `internal/transport/ssh.go`, `internal/transport/hostkeys.go`, `internal/testutil/sshserver.go`
- Test: `internal/transport/ssh_test.go`, `internal/transport/hostkeys_test.go`

**Interfaces:**
- Consumes: `transport.Session`, `transport.Options`, `transport.ErrAuth`, `shell.NewMarkerSession`, `testutil.ServeFakeShell`
- Produces:
  - `transport.DialSSH(ctx, addr, user, pass string, opt Options) (Session, error)`
  - `(*transport.SSHSession).Client() *ssh.Client` — used by sftp/scp uploaders via interface `interface{ Client() *ssh.Client }`
  - `testutil.FakeSSH{User, Pass string; NoExec bool; Handler func(cmd string) (string, int)}` + `(FakeSSH).Start(t *testing.T) string` — serves exec, pty+shell (marker fake shell), and the `sftp` subsystem on the real local filesystem

- [ ] **Step 1: Add dependencies**

```bash
cd DeviceFileUpdater
go get golang.org/x/crypto/ssh github.com/pkg/sftp
```

- [ ] **Step 2: Write fake SSH server**

`internal/testutil/sshserver.go`:
```go
package testutil

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FakeSSH struct {
	User, Pass string
	NoExec     bool // reject "exec" requests, forcing PTY shell fallback
	Handler    func(cmd string) (string, int)
}

func (f FakeSSH) Start(t *testing.T) string {
	t.Helper()
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, p []byte) (*ssh.Permissions, error) {
			if c.User() == f.User && string(p) == f.Pass {
				return nil, nil
			}
			return nil, fmt.Errorf("denied")
		},
	}
	cfg.AddHostKey(signer)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c, cfg)
		}
	}()
	return ln.Addr().String()
}

func (f FakeSSH) serve(c net.Conn, cfg *ssh.ServerConfig) {
	_, chans, reqs, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		c.Close()
		return
	}
	go ssh.DiscardRequests(reqs)
	for nc := range chans {
		if nc.ChannelType() != "session" {
			nc.Reject(ssh.UnknownChannelType, "no")
			continue
		}
		ch, creqs, err := nc.Accept()
		if err != nil {
			continue
		}
		go f.session(ch, creqs)
	}
}

func (f FakeSSH) session(ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			req.Reply(true, nil)
		case "exec":
			if f.NoExec {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			n := binary.BigEndian.Uint32(req.Payload[:4])
			out, code := f.Handler(string(req.Payload[4 : 4+n]))
			if out != "" {
				ch.Write([]byte(out + "\n"))
			}
			status := make([]byte, 4)
			binary.BigEndian.PutUint32(status, uint32(code))
			ch.SendRequest("exit-status", false, status)
			return
		case "shell":
			req.Reply(true, nil)
			go ServeFakeShell(ch, ch, true, f.Handler)
		case "subsystem":
			n := binary.BigEndian.Uint32(req.Payload[:4])
			if string(req.Payload[4:4+n]) != "sftp" {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			srv, err := sftp.NewServer(ch)
			if err != nil {
				return
			}
			srv.Serve()
			return
		default:
			req.Reply(false, nil)
		}
	}
}
```

- [ ] **Step 3: Write failing tests**

`internal/transport/ssh_test.go`:
```go
package transport

import (
	"context"
	"errors"
	"testing"

	"devupdater/internal/testutil"
)

func handler(cmd string) (string, int) {
	switch cmd {
	case "uname":
		return "Linux", 0
	case "fail":
		return "boom", 3
	}
	return "", 0
}

func TestSSHExec(t *testing.T) {
	for _, noExec := range []bool{false, true} {
		addr := testutil.FakeSSH{User: "dev", Pass: "pw", NoExec: noExec, Handler: handler}.Start(t)
		s, err := DialSSH(context.Background(), addr, "dev", "pw", testOpts())
		if err != nil {
			t.Fatalf("noExec=%v: %v", noExec, err)
		}
		out, code, err := s.Exec(context.Background(), "uname")
		if err != nil || code != 0 || out != "Linux" {
			t.Fatalf("noExec=%v: out=%q code=%d err=%v", noExec, out, code, err)
		}
		out, code, _ = s.Exec(context.Background(), "fail")
		if code != 3 || out != "boom" {
			t.Fatalf("noExec=%v fail: out=%q code=%d", noExec, out, code)
		}
		if s.Protocol() != "ssh" {
			t.Fatal(s.Protocol())
		}
		s.Close()
	}
}

func TestSSHBadPassword(t *testing.T) {
	addr := testutil.FakeSSH{User: "dev", Pass: "pw", Handler: handler}.Start(t)
	_, err := DialSSH(context.Background(), addr, "dev", "nope", testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}
```

`internal/transport/hostkeys_test.go`:
```go
package transport

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func newKey(t *testing.T) ssh.PublicKey {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestTOFU(t *testing.T) {
	p := filepath.Join(t.TempDir(), "known_hosts")
	k1, k2 := newKey(t), newKey(t)
	if err := checkKnownHost(p, "10.0.0.1:22", k1); err != nil {
		t.Fatal(err)
	}
	if err := checkKnownHost(p, "10.0.0.1:22", k1); err != nil {
		t.Fatal("same key must pass:", err)
	}
	if err := checkKnownHost(p, "10.0.0.1:22", k2); err == nil {
		t.Fatal("changed key must fail")
	}
	if err := checkKnownHost(p, "10.0.0.2:22", k2); err != nil {
		t.Fatal("other host must pass:", err)
	}
}
```

- [ ] **Step 4: Run, verify failure**

Run: `go test ./internal/transport/`
Expected: FAIL — `undefined: DialSSH`, `undefined: checkKnownHost`.

- [ ] **Step 5: Implement**

`internal/transport/hostkeys.go`:
```go
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
```

`internal/transport/ssh.go`:
```go
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"devupdater/internal/shell"

	"golang.org/x/crypto/ssh"
)

type SSHSession struct {
	client *ssh.Client
	pty    *shell.MarkerSession // non-nil when the server refuses exec
}

func DialSSH(ctx context.Context, addr, user, pass string, opt Options) (Session, error) {
	cfg := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(pass),
			ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
				ans := make([]string, len(qs))
				for i := range ans {
					ans[i] = pass
				}
				return ans, nil
			}),
		},
		HostKeyCallback: hostKeyCallback(opt),
		Timeout:         opt.ConnectTimeout,
	}
	d := net.Dialer{Timeout: opt.ConnectTimeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh connect %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(opt.ConnectTimeout))
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		conn.Close()
		if strings.Contains(err.Error(), "unable to authenticate") {
			return nil, fmt.Errorf("ssh %s: %w", addr, ErrAuth)
		}
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	conn.SetDeadline(time.Time{})
	s := &SSHSession{client: ssh.NewClient(c, chans, reqs)}

	cctx, cancel := context.WithTimeout(ctx, opt.CommandTimeout)
	defer cancel()
	if _, code, err := s.execChannel(cctx, "true"); err == nil && code == 0 {
		return s, nil
	}
	if err := s.startPTY(cctx); err != nil {
		s.client.Close()
		return nil, fmt.Errorf("ssh %s: neither exec nor shell works: %w", addr, err)
	}
	return s, nil
}

func (s *SSHSession) startPTY(ctx context.Context) error {
	sess, err := s.client.NewSession()
	if err != nil {
		return err
	}
	if err := sess.RequestPty("vt100", 40, 1000, ssh.TerminalModes{ssh.ECHO: 0}); err != nil {
		return err
	}
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	if err := sess.Shell(); err != nil {
		return err
	}
	m := shell.NewMarkerSession(stdout, stdin, "\n")
	if err := m.Init(ctx); err != nil {
		return err
	}
	s.pty = m
	return nil
}

func (s *SSHSession) execChannel(ctx context.Context, cmd string) (string, int, error) {
	sess, err := s.client.NewSession()
	if err != nil {
		return "", -1, err
	}
	defer sess.Close()
	var buf bytes.Buffer
	sess.Stdout, sess.Stderr = &buf, &buf
	if err := sess.Start(cmd); err != nil {
		return "", -1, err
	}
	done := make(chan error, 1)
	go func() { done <- sess.Wait() }()
	select {
	case err := <-done:
		code := 0
		if err != nil {
			var ee *ssh.ExitError
			if !errors.As(err, &ee) {
				return buf.String(), -1, err
			}
			code = ee.ExitStatus()
		}
		return strings.TrimSuffix(buf.String(), "\n"), code, nil
	case <-ctx.Done():
		sess.Signal(ssh.SIGKILL)
		return "", -1, ctx.Err()
	}
}

func (s *SSHSession) Exec(ctx context.Context, cmd string) (string, int, error) {
	if s.pty != nil {
		return s.pty.Exec(ctx, cmd)
	}
	return s.execChannel(ctx, cmd)
}

func (s *SSHSession) Client() *ssh.Client { return s.client }
func (s *SSHSession) Protocol() string    { return "ssh" }
func (s *SSHSession) Close() error        { return s.client.Close() }
```

- [ ] **Step 6: Run, verify pass**

Run: `go test -race ./internal/transport/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/transport internal/testutil
git commit -m "feat: ssh transport with exec, pty fallback and TOFU host keys"
```

---

### Task 6: Dial — SSH first, Telnet fallback, one retry

**Files:**
- Create: `internal/transport/dial.go`
- Test: `internal/transport/dial_test.go`

**Interfaces:**
- Consumes: `DialSSH`, `DialTelnet`, `ErrAuth`, `workspace.Device` (`Addr(defaultPort)`)
- Produces: `transport.Dial(ctx context.Context, d workspace.Device, opt Options) (Session, error)`

Rules (spec §6): try SSH on `d.Addr(22)`; on any SSH error try Telnet on `d.Addr(23)`. A connection-level failure (not `ErrAuth`) of both is retried once after 1s. The final error contains both attempts' messages.

- [ ] **Step 1: Write failing tests**

`internal/transport/dial_test.go`:
```go
package transport

import (
	"context"
	"errors"
	"strings"
	"testing"

	"devupdater/internal/testutil"
	"devupdater/internal/workspace"
)

func TestDialPrefersSSH(t *testing.T) {
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: handler}.Start(t)
	s, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "p"}, testOpts())
	if err != nil || s.Protocol() != "ssh" {
		t.Fatalf("s=%v err=%v", s, err)
	}
	s.Close()
}

func TestDialFallsBackToTelnet(t *testing.T) {
	// Same port for both attempts: the SSH handshake fails against a telnet server.
	addr := testutil.StartFakeTelnet(t, "u", "p", handler)
	s, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "p"}, testOpts())
	if err != nil || s.Protocol() != "telnet" {
		t.Fatalf("s=%v err=%v", s, err)
	}
	s.Close()
}

func TestDialBothFail(t *testing.T) {
	_, err := Dial(context.Background(), workspace.Device{Host: "127.0.0.1:1", Username: "u", Password: "p"}, testOpts())
	if err == nil || !strings.Contains(err.Error(), "ssh") || !strings.Contains(err.Error(), "telnet") {
		t.Fatalf("expected combined error, got %v", err)
	}
}

func TestDialAuthNotRetried(t *testing.T) {
	addr := testutil.StartFakeTelnet(t, "u", "p", handler)
	_, err := Dial(context.Background(), workspace.Device{Host: addr, Username: "u", Password: "bad"}, testOpts())
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/transport/ -run Dial`
Expected: FAIL — `undefined: Dial`.

- [ ] **Step 3: Implement**

`internal/transport/dial.go`:
```go
package transport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"devupdater/internal/workspace"
)

// Dial connects with SSH, falling back to Telnet, retrying once on connection errors.
func Dial(ctx context.Context, d workspace.Device, opt Options) (Session, error) {
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		var s Session
		s, err = dialOnce(ctx, d, opt)
		if err == nil {
			return s, nil
		}
		if errors.Is(err, ErrAuth) || ctx.Err() != nil {
			return nil, err
		}
	}
	return nil, err
}

func dialOnce(ctx context.Context, d workspace.Device, opt Options) (Session, error) {
	s, sshErr := DialSSH(ctx, d.Addr(22), d.Username, d.Password, opt)
	if sshErr == nil {
		return s, nil
	}
	s, telErr := DialTelnet(ctx, d.Addr(23), d.Username, d.Password, opt)
	if telErr == nil {
		return s, nil
	}
	if errors.Is(sshErr, ErrAuth) || errors.Is(telErr, ErrAuth) {
		return nil, fmt.Errorf("%w (ssh: %v; telnet: %v)", ErrAuth, sshErr, telErr)
	}
	return nil, fmt.Errorf("ssh: %v; telnet: %v", sshErr, telErr)
}
```

- [ ] **Step 4: Run, verify pass**

Run: `go test -race ./internal/transport/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transport
git commit -m "feat: dial devices with ssh-then-telnet fallback and retry"
```

---

### Task 7: Test sessions + probe

**Files:**
- Create: `internal/testutil/sessions.go`, `internal/testutil/localshell.go`, `internal/probe/probe.go`
- Test: `internal/probe/probe_test.go`

**Interfaces:**
- Consumes: `transport.Session`, `workspace.Device`
- Produces:
  - `testutil.FakeSession{Proto string; Handler func(cmd string) (string, int); Cmds []string}` implementing `transport.Session` (records every command in `Cmds`)
  - `testutil.LocalShell{}` implementing `transport.Session` by running `sh -c cmd` on the test machine (`//go:build !windows`)
  - `probe.Tools` = `[]string{"sha256sum","md5sum","base64","od","hexdump","printf","stat","scp"}`
  - `probe.Caps{Tools map[string]bool; SFTP, FTP bool}` + `(Caps).Has(name string) bool` + `(Caps).List() []string` (sorted names incl. `sftp`/`ftp`)
  - `probe.Probe(ctx context.Context, s transport.Session, ftpHost string, timeout time.Duration) (Caps, error)`

SFTP availability is checked by the upload chain itself (Task 9) rather than here; `Probe` sets `SFTP` only when the session exposes `Client() *ssh.Client` and opening an sftp client succeeds. `FTP` = TCP connect to `ftpHost:21` within `timeout` (max 3s) succeeds.

- [ ] **Step 1: Write test sessions**

`internal/testutil/sessions.go`:
```go
package testutil

import (
	"context"
	"sync"
)

// FakeSession answers commands from Handler and records them.
type FakeSession struct {
	Proto   string
	Handler func(cmd string) (string, int)
	mu      sync.Mutex
	Cmds    []string
}

func (f *FakeSession) Exec(_ context.Context, cmd string) (string, int, error) {
	f.mu.Lock()
	f.Cmds = append(f.Cmds, cmd)
	f.mu.Unlock()
	out, code := f.Handler(cmd)
	return out, code, nil
}

func (f *FakeSession) Protocol() string {
	if f.Proto == "" {
		return "telnet"
	}
	return f.Proto
}

func (f *FakeSession) Close() error { return nil }
```

`internal/testutil/localshell.go`:
```go
//go:build !windows

package testutil

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// LocalShell runs commands with the machine's /bin/sh — a stand-in device for tests.
type LocalShell struct{}

func (LocalShell) Exec(ctx context.Context, cmd string) (string, int, error) {
	out, err := exec.CommandContext(ctx, "/bin/sh", "-c", cmd).CombinedOutput()
	s := strings.TrimSuffix(string(out), "\n")
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return s, ee.ExitCode(), nil
	}
	if err != nil {
		return s, -1, err
	}
	return s, 0, nil
}

func (LocalShell) Protocol() string { return "local" }
func (LocalShell) Close() error     { return nil }
```

- [ ] **Step 2: Write failing tests**

`internal/probe/probe_test.go`:
```go
package probe

import (
	"context"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func TestProbeParsesTools(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\nHAVE printf\ngarbage\nHAVE od", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("md5sum") || !c.Has("printf") || !c.Has("od") || c.Has("base64") || c.SFTP {
		t.Fatalf("caps %+v", c)
	}
	if !strings.Contains(s.Cmds[0], "command -v") {
		t.Fatalf("probe cmd %q", s.Cmds[0])
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run: `go test ./internal/probe/`
Expected: FAIL — `undefined: Probe`.

- [ ] **Step 4: Implement**

`internal/probe/probe.go`:
```go
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
```

- [ ] **Step 5: Run, verify pass**

Run: `go test -race ./internal/probe/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/testutil internal/probe
git commit -m "feat: probe device tools, sftp and ftp availability"
```

---

### Task 8: Uploader interface, fallback chain, shell uploaders

**Files:**
- Create: `internal/upload/upload.go`, `internal/upload/shell.go`
- Test: `internal/upload/chain_test.go`, `internal/upload/shell_test.go`

**Interfaces:**
- Consumes: `transport.Session`, `shell.Quote`, `testutil.LocalShell`
- Produces:
  - `upload.Uploader` interface: `Name() string`, `Upload(ctx context.Context, data []byte, remote string) error` (must create or truncate `remote`)
  - `upload.NewChain(us []Uploader) *Chain`; `(*Chain).Upload(ctx, data, remote) (method string, err error)`; `(*Chain).Primary() string`
  - `upload.NewShellBase64(s transport.Session, cmdTimeout time.Duration) Uploader` (name `shell-base64`)
  - `upload.NewShellPrintf(s transport.Session, cmdTimeout time.Duration) Uploader` (name `shell-printf`)
  - `upload.run(ctx, s, timeout, label, cmd string) error` (package-internal helper: non-zero exit → error)

- [ ] **Step 1: Write failing tests**

`internal/upload/chain_test.go`:
```go
package upload

import (
	"context"
	"errors"
	"testing"
)

type fakeUp struct {
	name  string
	fail  bool
	calls int
}

func (f *fakeUp) Name() string { return f.name }
func (f *fakeUp) Upload(context.Context, []byte, string) error {
	f.calls++
	if f.fail {
		return errors.New(f.name + " broken")
	}
	return nil
}

func TestChainFallsBackAndRemembers(t *testing.T) {
	a, b := &fakeUp{name: "sftp", fail: true}, &fakeUp{name: "shell-printf"}
	c := NewChain([]Uploader{a, b})
	if c.Primary() != "sftp" {
		t.Fatal(c.Primary())
	}
	for i := 0; i < 2; i++ {
		m, err := c.Upload(context.Background(), []byte("x"), "/t")
		if err != nil || m != "shell-printf" {
			t.Fatalf("m=%s err=%v", m, err)
		}
	}
	if a.calls != 1 {
		t.Fatalf("broken uploader retried: %d calls", a.calls)
	}
	if c.Primary() != "shell-printf" {
		t.Fatal(c.Primary())
	}
}

func TestChainAllFail(t *testing.T) {
	c := NewChain([]Uploader{&fakeUp{name: "a", fail: true}, &fakeUp{name: "b", fail: true}})
	if _, err := c.Upload(context.Background(), nil, "/t"); err == nil {
		t.Fatal("expected error")
	}
}

func TestChainStopsOnCancel(t *testing.T) {
	a := &fakeUp{name: "a", fail: true}
	c := NewChain([]Uploader{a, &fakeUp{name: "b"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Upload(ctx, nil, "/t"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancel, got %v", err)
	}
}
```

`internal/upload/shell_test.go`:
```go
//go:build !windows

package upload

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func allBytes() []byte {
	var b bytes.Buffer
	for i := 0; i < 12; i++ {
		for c := 0; c < 256; c++ {
			b.WriteByte(byte(c))
		}
	}
	b.WriteString("-%s'\\ end")
	return b.Bytes()
}

func TestShellUploadersRoundTrip(t *testing.T) {
	for _, u := range []Uploader{
		NewShellBase64(testutil.LocalShell{}, 5*time.Second),
		NewShellPrintf(testutil.LocalShell{}, 5*time.Second),
	} {
		for _, data := range [][]byte{allBytes(), {}, []byte("-leading dash")} {
			dst := filepath.Join(t.TempDir(), "sub dir", "out.bin")
			os.MkdirAll(filepath.Dir(dst), 0o755)
			if err := u.Upload(context.Background(), data, dst); err != nil {
				t.Fatalf("%s: %v", u.Name(), err)
			}
			got, _ := os.ReadFile(dst)
			if !bytes.Equal(got, data) {
				t.Fatalf("%s: content mismatch (len %d vs %d)", u.Name(), len(got), len(data))
			}
			if _, err := os.Stat(dst + ".b64"); err == nil {
				t.Fatalf("%s: leftover .b64", u.Name())
			}
		}
	}
}

func TestPrintfCommandsStayShort(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	NewShellPrintf(s, time.Second).Upload(context.Background(), allBytes(), "/x")
	for _, c := range s.Cmds {
		if len(c) > 1024 {
			t.Fatalf("command too long: %d", len(c))
		}
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/upload/`
Expected: FAIL — `undefined: NewChain`.

- [ ] **Step 3: Implement**

`internal/upload/upload.go`:
```go
// Package upload writes file content to a device path using the best available method.
package upload

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/transport"
)

type Uploader interface {
	Name() string
	// Upload creates or truncates remote and writes data to it.
	Upload(ctx context.Context, data []byte, remote string) error
}

// Chain tries uploaders in order. One that fails is skipped for the rest of the device run.
type Chain struct {
	list   []Uploader
	broken map[string]bool
}

func NewChain(us []Uploader) *Chain { return &Chain{list: us, broken: map[string]bool{}} }

func (c *Chain) Primary() string {
	for _, u := range c.list {
		if !c.broken[u.Name()] {
			return u.Name()
		}
	}
	return ""
}

func (c *Chain) Upload(ctx context.Context, data []byte, remote string) (string, error) {
	var errs []string
	for _, u := range c.list {
		if c.broken[u.Name()] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return "", err
		}
		err := u.Upload(ctx, data, remote)
		if err == nil {
			return u.Name(), nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		c.broken[u.Name()] = true
		errs = append(errs, u.Name()+": "+err.Error())
	}
	if len(errs) == 0 {
		return "", errors.New("no upload method available")
	}
	return "", fmt.Errorf("all upload methods failed: %s", strings.Join(errs, "; "))
}

// run executes cmd with its own timeout; a non-zero exit becomes an error labelled with label.
func run(ctx context.Context, s transport.Session, timeout time.Duration, label, cmd string) error {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, code, err := s.Exec(c, cmd)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	if code != 0 {
		return fmt.Errorf("%s: exit %d: %s", label, code, strings.TrimSpace(out))
	}
	return nil
}
```

`internal/upload/shell.go`:
```go
package upload

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

// Keeps every command line under the 1024-byte limit including quoting and path.
const maxPayload = 700

type shellBase64 struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellBase64(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellBase64{s: s, timeout: cmdTimeout}
}

func (u *shellBase64) Name() string { return "shell-base64" }

func (u *shellBase64) Upload(ctx context.Context, data []byte, remote string) error {
	b64 := shell.Quote(remote + ".b64")
	dst := shell.Quote(remote)
	if err := run(ctx, u.s, u.timeout, "truncate", ": > "+b64); err != nil {
		return err
	}
	enc := base64.StdEncoding.EncodeToString(data)
	for len(enc) > 0 {
		n := min(maxPayload, len(enc))
		if err := run(ctx, u.s, u.timeout, "write chunk", "printf '%s' "+shell.Quote(enc[:n])+" >> "+b64); err != nil {
			run(context.Background(), u.s, u.timeout, "cleanup", "rm -f "+b64)
			return err
		}
		enc = enc[n:]
	}
	return run(ctx, u.s, u.timeout, "base64 decode",
		fmt.Sprintf("if base64 -d < %s > %s; then rm -f %s; else rm -f %s; false; fi", b64, dst, b64, b64))
}

type shellPrintf struct {
	s       transport.Session
	timeout time.Duration
}

func NewShellPrintf(s transport.Session, cmdTimeout time.Duration) Uploader {
	return &shellPrintf{s: s, timeout: cmdTimeout}
}

func (u *shellPrintf) Name() string { return "shell-printf" }

// printfEscape turns bytes into a printf format string that reproduces them exactly.
func printfEscape(b byte) string {
	if b >= 0x21 && b <= 0x7e && b != '\\' && b != '%' && b != '\'' && b != '-' {
		return string(b)
	}
	return fmt.Sprintf(`\%03o`, b)
}

func (u *shellPrintf) Upload(ctx context.Context, data []byte, remote string) error {
	dst := shell.Quote(remote)
	if len(data) == 0 {
		return run(ctx, u.s, u.timeout, "truncate", ": > "+dst)
	}
	redirect := ">"
	var chunk strings.Builder
	flush := func() error {
		err := run(ctx, u.s, u.timeout, "write chunk", "printf '"+chunk.String()+"' "+redirect+" "+dst)
		chunk.Reset()
		redirect = ">>"
		return err
	}
	for _, b := range data {
		e := printfEscape(b)
		if chunk.Len()+len(e) > maxPayload {
			if err := flush(); err != nil {
				return err
			}
		}
		chunk.WriteString(e)
	}
	return flush()
}
```

- [ ] **Step 4: Run, verify pass**

Run: `go test -race ./internal/upload/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/upload
git commit -m "feat: upload fallback chain with shell-base64 and shell-printf"
```

---

### Task 9: SFTP, SCP, FTP uploaders + method selection

**Files:**
- Create: `internal/upload/sftp.go`, `internal/upload/scp.go`, `internal/upload/ftp.go`, `internal/upload/select.go`
- Test: `internal/upload/sftp_test.go`, `internal/upload/select_test.go`

**Interfaces:**
- Consumes: `Uploader`, `run`, `NewShellBase64`, `NewShellPrintf`, `probe.Caps`, `transport.DialSSH`, `testutil.FakeSSH`
- Produces:
  - `upload.NewSFTP(c *ssh.Client) Uploader` (`sftp`), `upload.NewSCP(c *ssh.Client) Uploader` (`scp`)
  - `upload.FTPCreds{Host, User, Pass string}`; `upload.NewFTP(s transport.Session, cr FTPCreds, cmdTimeout time.Duration) Uploader` (`ftp`; verifies size with `wc -c` over the session because an ftpd chroot could put the file elsewhere)
  - `upload.Select(s transport.Session, c probe.Caps, cr FTPCreds, cmdTimeout time.Duration) []Uploader` — order sftp, scp, ftp, shell-base64, shell-printf (shell-printf always last)

SCP and FTP are exercised by the Docker integration suite (Task 14); no unit fake is worth its weight for them.

- [ ] **Step 1: Add dependency**

```bash
cd DeviceFileUpdater
go get github.com/jlaffaye/ftp
```

- [ ] **Step 2: Write failing tests**

`internal/upload/sftp_test.go`:
```go
//go:build !windows

package upload

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devupdater/internal/testutil"
	"devupdater/internal/transport"

	"golang.org/x/crypto/ssh"
)

func TestSFTPUpload(t *testing.T) {
	addr := testutil.FakeSSH{User: "u", Pass: "p", Handler: func(string) (string, int) { return "", 0 }}.Start(t)
	s, err := transport.DialSSH(context.Background(), addr, "u", "p",
		transport.Options{ConnectTimeout: 2 * time.Second, CommandTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	dst := filepath.Join(t.TempDir(), "f.bin")
	os.WriteFile(dst, []byte("old content that is longer"), 0o644)
	u := NewSFTP(s.(interface{ Client() *ssh.Client }).Client())
	if err := u.Upload(context.Background(), []byte("new"), dst); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new" {
		t.Fatalf("got %q (must truncate)", got)
	}
}
```

`internal/upload/select_test.go`:
```go
package upload

import (
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"
)

func names(us []Uploader) []string {
	var out []string
	for _, u := range us {
		out = append(out, u.Name())
	}
	return out
}

func TestSelectOrderNonSSH(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	c := probe.Caps{Tools: map[string]bool{"base64": true, "scp": true}, FTP: true, SFTP: true}
	got := names(Select(s, c, FTPCreds{}, time.Second))
	want := []string{"ftp", "shell-base64", "shell-printf"} // no ssh client → no sftp/scp
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestSelectMinimal(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) { return "", 0 }}
	got := names(Select(s, probe.Caps{Tools: map[string]bool{}}, FTPCreds{}, time.Second))
	if len(got) != 1 || got[0] != "shell-printf" {
		t.Fatalf("got %v", got)
	}
}
```

- [ ] **Step 3: Run, verify failure**

Run: `go test ./internal/upload/`
Expected: FAIL — `undefined: NewSFTP`, `undefined: Select`.

- [ ] **Step 4: Implement**

`internal/upload/sftp.go`:
```go
package upload

import (
	"context"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// doCtx runs fn, calling abort (which must unblock fn) if ctx ends first.
func doCtx(ctx context.Context, abort func(), fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		abort()
		<-done
		return ctx.Err()
	}
}

type sftpUp struct{ c *ssh.Client }

func NewSFTP(c *ssh.Client) Uploader { return &sftpUp{c: c} }
func (u *sftpUp) Name() string      { return "sftp" }

func (u *sftpUp) Upload(ctx context.Context, data []byte, remote string) error {
	sc, err := sftp.NewClient(u.c)
	if err != nil {
		return err
	}
	defer sc.Close()
	return doCtx(ctx, func() { sc.Close() }, func() error {
		f, err := sc.OpenFile(remote, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
		if err != nil {
			return err
		}
		if _, err := f.Write(data); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	})
}
```

`internal/upload/scp.go`:
```go
package upload

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"path"

	"devupdater/internal/shell"

	"golang.org/x/crypto/ssh"
)

type scpUp struct{ c *ssh.Client }

func NewSCP(c *ssh.Client) Uploader { return &scpUp{c: c} }
func (u *scpUp) Name() string      { return "scp" }

// Upload speaks the scp sink protocol against `scp -t <remote>`.
func (u *scpUp) Upload(ctx context.Context, data []byte, remote string) error {
	sess, err := u.c.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	stdoutPipe, err := sess.StdoutPipe()
	if err != nil {
		return err
	}
	stdout := bufio.NewReader(stdoutPipe)
	ack := func() error {
		b, err := stdout.ReadByte()
		if err != nil {
			return err
		}
		if b != 0 {
			msg, _ := stdout.ReadString('\n')
			return fmt.Errorf("scp: %s", msg)
		}
		return nil
	}
	return doCtx(ctx, func() { sess.Close() }, func() error {
		if err := sess.Start("scp -t " + shell.Quote(remote)); err != nil {
			return err
		}
		if err := ack(); err != nil {
			return err
		}
		fmt.Fprintf(stdin, "C0644 %d %s\n", len(data), path.Base(remote))
		if err := ack(); err != nil {
			return err
		}
		stdin.Write(data)
		stdin.Write([]byte{0})
		if err := ack(); err != nil {
			return err
		}
		stdin.Close()
		io.Copy(io.Discard, stdout)
		return sess.Wait()
	})
}
```

`internal/upload/ftp.go`:
```go
package upload

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"devupdater/internal/shell"
	"devupdater/internal/transport"

	"github.com/jlaffaye/ftp"
)

type FTPCreds struct{ Host, User, Pass string }

type ftpUp struct {
	s       transport.Session
	cr      FTPCreds
	timeout time.Duration
}

func NewFTP(s transport.Session, cr FTPCreds, cmdTimeout time.Duration) Uploader {
	return &ftpUp{s: s, cr: cr, timeout: cmdTimeout}
}

func (u *ftpUp) Name() string { return "ftp" }

func (u *ftpUp) Upload(ctx context.Context, data []byte, remote string) error {
	c, err := ftp.Dial(net.JoinHostPort(u.cr.Host, "21"), ftp.DialWithContext(ctx), ftp.DialWithTimeout(u.timeout))
	if err != nil {
		return err
	}
	defer c.Quit()
	err = doCtx(ctx, func() { c.Quit() }, func() error {
		if err := c.Login(u.cr.User, u.cr.Pass); err != nil {
			return err
		}
		return c.Stor(remote, bytes.NewReader(data))
	})
	if err != nil {
		return err
	}
	// The ftpd root may not be "/": confirm the file really landed at remote.
	cctx, cancel := context.WithTimeout(ctx, u.timeout)
	defer cancel()
	out, code, err := u.s.Exec(cctx, "wc -c < "+shell.Quote(remote))
	if err != nil {
		return err
	}
	if n, _ := strconv.Atoi(strings.TrimSpace(out)); code != 0 || n != len(data) {
		return fmt.Errorf("ftp: file not found at %s after upload (ftpd root is not /?)", remote)
	}
	return nil
}
```

`internal/upload/select.go`:
```go
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
		if c.SFTP {
			us = append(us, NewSFTP(cs.Client()))
		}
		if c.Has("scp") {
			us = append(us, NewSCP(cs.Client()))
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
```

- [ ] **Step 5: Run, verify pass**

Run: `go test -race ./internal/upload/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/upload
git commit -m "feat: sftp/scp/ftp uploaders and preference-ordered selection"
```

---

### Task 10: Syncer — hashing, decision, atomic write

**Files:**
- Create: `internal/syncer/hasher.go`, `internal/syncer/remote.go`, `internal/syncer/syncer.go`
- Test: `internal/syncer/hasher_test.go`, `internal/syncer/remote_test.go`, `internal/syncer/syncer_test.go`

**Interfaces:**
- Consumes: `transport.Session`, `probe.Caps`, `upload.Chain`, `upload.NewShellPrintf`, `shell.Quote`, `model.LocalFile`, `model.FileResult`, `testutil.LocalShell`, `testutil.FakeSession`
- Produces:
  - `syncer.Hasher` interface: `Name() string`, `Remote(ctx, path string) (string, error)`, `Local(data []byte) string`
  - `syncer.SelectHasher(s transport.Session, c probe.Caps, cmdTimeout time.Duration) Hasher` (nil when nothing usable)
  - `syncer.Device{S transport.Session; Hasher Hasher; Chain *upload.Chain; CmdTimeout time.Duration}`
  - `syncer.Options{DryRun, Backup bool}`
  - `(*syncer.Device).SyncFile(ctx context.Context, f model.LocalFile, opt Options) model.FileResult`
  - `syncer.NoteNotCompared = "not compared: device has no hash tool"`, `syncer.NoteModeUnknown = "existing mode unreadable, used 0644"`

- [ ] **Step 1: Write failing tests**

`internal/syncer/hasher_test.go`:
```go
package syncer

import (
	"context"
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"
)

func TestSelectHasherOrder(t *testing.T) {
	s := &testutil.FakeSession{}
	cases := []struct {
		tools []string
		want  string
	}{
		{[]string{"md5sum", "sha256sum", "base64"}, "sha256sum"},
		{[]string{"md5sum", "od"}, "md5sum"},
		{[]string{"od", "base64"}, "readback-base64"},
		{[]string{"hexdump", "od"}, "readback-od"},
		{[]string{"hexdump"}, "readback-hexdump"},
	}
	for _, c := range cases {
		caps := probe.Caps{Tools: map[string]bool{}}
		for _, t := range c.tools {
			caps.Tools[t] = true
		}
		if h := SelectHasher(s, caps, time.Second); h == nil || h.Name() != c.want {
			t.Errorf("%v: got %v want %s", c.tools, h, c.want)
		}
	}
	if SelectHasher(s, probe.Caps{Tools: map[string]bool{}}, time.Second) != nil {
		t.Error("expected nil hasher")
	}
}

func TestSumHasherParses(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) {
		return "5D41402ABC4B2A76B9719D911017C592  /x", 0
	}}
	h := SelectHasher(s, probe.Caps{Tools: map[string]bool{"md5sum": true}}, time.Second)
	got, err := h.Remote(context.Background(), "/x")
	if err != nil || got != h.Local([]byte("hello")) {
		t.Fatalf("got %q err %v, local %q", got, err, h.Local([]byte("hello")))
	}
}

func TestReadbackOd(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) {
		return " 68 65 6c\n 6c 6f", 0
	}}
	h := SelectHasher(s, probe.Caps{Tools: map[string]bool{"od": true}}, time.Second)
	got, err := h.Remote(context.Background(), "/x")
	if err != nil || got != h.Local([]byte("hello")) {
		t.Fatalf("got %q err %v", got, err)
	}
}
```

`internal/syncer/remote_test.go`:
```go
package syncer

import "testing"

func TestParseLsMode(t *testing.T) {
	cases := map[string]string{
		"-rwxr-xr-x 1 0 0 12 Jan 1 00:00 /x": "0755",
		"-rw-r--r-- 1 0 0 12 Jan 1 00:00 /x": "0644",
		"-rw------- 1 0 0 12 Jan 1 00:00 /x": "0600",
		"-rwsr-xr-x 1 0 0 12 Jan 1 00:00 /x": "0755",
	}
	for in, want := range cases {
		if got, ok := parseMode(in); !ok || got != want {
			t.Errorf("%q: got %q ok=%v want %s", in, got, ok, want)
		}
	}
	if got, ok := parseMode("755"); !ok || got != "0755" {
		t.Errorf("stat form: %q", got)
	}
	if _, ok := parseMode("garbage"); ok {
		t.Error("garbage must fail")
	}
}
```

`internal/syncer/syncer_test.go`:
```go
//go:build !windows

package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/upload"
)

func newDev(withHash bool) *Device {
	s := testutil.LocalShell{}
	var h Hasher
	if withHash {
		h = SelectHasher(s, probe.Caps{Tools: map[string]bool{"base64": true}}, 5*time.Second)
	}
	return &Device{
		S: s, Hasher: h, CmdTimeout: 5 * time.Second,
		Chain: upload.NewChain([]upload.Uploader{upload.NewShellPrintf(s, 5*time.Second)}),
	}
}

func lf(remote, content, mode string) model.LocalFile {
	sum := sha256.Sum256([]byte(content))
	return model.LocalFile{Remote: remote, Mode: mode, Data: []byte(content), SHA256: hex.EncodeToString(sum[:])}
}

func modeOf(t *testing.T, p string) os.FileMode {
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode().Perm()
}

func TestCreateUnchangedUpdate(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "new", "dir", "app.conf")
	ctx := context.Background()

	r := d.SyncFile(ctx, lf(dst, "v1", ""), Options{Backup: true})
	if r.Status != model.Created || r.Method != "shell-printf" {
		t.Fatalf("create: %+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "v1" || modeOf(t, dst) != 0o644 {
		t.Fatalf("create content/mode wrong: %q %v", got, modeOf(t, dst))
	}

	r = d.SyncFile(ctx, lf(dst, "v1", ""), Options{Backup: true})
	if r.Status != model.Unchanged || r.Method != "" {
		t.Fatalf("unchanged: %+v", r)
	}

	os.Chmod(dst, 0o755)
	r = d.SyncFile(ctx, lf(dst, "v2", ""), Options{Backup: true})
	if r.Status != model.Updated {
		t.Fatalf("update: %+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "v2" || modeOf(t, dst) != 0o755 {
		t.Fatalf("update content/mode wrong: %q %v", got, modeOf(t, dst))
	}
	if bak, _ := os.ReadFile(dst + ".bak"); string(bak) != "v1" {
		t.Fatalf("backup: %q", bak)
	}
	if _, err := os.Stat(dst + ".devupd.tmp"); err == nil {
		t.Fatal("tmp left behind")
	}
}

func TestExplicitModeAndNoBackup(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "x")
	os.WriteFile(dst, []byte("old"), 0o644)
	r := d.SyncFile(context.Background(), lf(dst, "new", "0600"), Options{Backup: false})
	if r.Status != model.Updated || modeOf(t, dst) != 0o600 {
		t.Fatalf("%+v mode %v", r, modeOf(t, dst))
	}
	if _, err := os.Stat(dst + ".bak"); err == nil {
		t.Fatal("backup must not exist")
	}
}

func TestDryRun(t *testing.T) {
	d := newDev(true)
	dir := t.TempDir()
	existing := filepath.Join(dir, "e")
	os.WriteFile(existing, []byte("old"), 0o644)
	r1 := d.SyncFile(context.Background(), lf(filepath.Join(dir, "n"), "x", ""), Options{DryRun: true})
	r2 := d.SyncFile(context.Background(), lf(existing, "new", ""), Options{DryRun: true})
	if r1.Status != model.WouldCreate || r2.Status != model.WouldUpdate {
		t.Fatalf("%+v %+v", r1, r2)
	}
	if _, err := os.Stat(filepath.Join(dir, "n")); err == nil {
		t.Fatal("dry-run created a file")
	}
	if got, _ := os.ReadFile(existing); string(got) != "old" {
		t.Fatal("dry-run modified a file")
	}
}

func TestNoHasherAlwaysWrites(t *testing.T) {
	d := newDev(false)
	dst := filepath.Join(t.TempDir(), "x")
	os.WriteFile(dst, []byte("same"), 0o644)
	r := d.SyncFile(context.Background(), lf(dst, "same", ""), Options{})
	if r.Status != model.Updated || r.Note != NoteNotCompared {
		t.Fatalf("%+v", r)
	}
}

func TestUnwritableTargetFailsCleanly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	d := newDev(true)
	dir := filepath.Join(t.TempDir(), "ro")
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, "x")
	os.WriteFile(dst, []byte("orig"), 0o644)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	r := d.SyncFile(context.Background(), lf(dst, "new", ""), Options{Backup: true})
	if r.Status != model.Failed || r.Error == "" {
		t.Fatalf("%+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "orig" {
		t.Fatal("original modified")
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/syncer/`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement**

`internal/syncer/hasher.go`:
```go
// Package syncer decides what to do with each file on a device and performs the write.
package syncer

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/shell"
	"devupdater/internal/transport"
)

// Hasher yields a digest of a remote file comparable with Local(data).
type Hasher interface {
	Name() string
	Remote(ctx context.Context, path string) (string, error)
	Local(data []byte) string
}

func SelectHasher(s transport.Session, c probe.Caps, cmdTimeout time.Duration) Hasher {
	switch {
	case c.Has("sha256sum"):
		return &sumHasher{s, cmdTimeout, "sha256sum"}
	case c.Has("md5sum"):
		return &sumHasher{s, cmdTimeout, "md5sum"}
	case c.Has("base64"):
		return &readback{s, cmdTimeout, "base64"}
	case c.Has("od"):
		return &readback{s, cmdTimeout, "od"}
	case c.Has("hexdump"):
		return &readback{s, cmdTimeout, "hexdump"}
	}
	return nil
}

func exec(ctx context.Context, s transport.Session, timeout time.Duration, cmd string) (string, error) {
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	out, code, err := s.Exec(c, cmd)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("exit %d: %s", code, strings.TrimSpace(out))
	}
	return out, nil
}

type sumHasher struct {
	s       transport.Session
	timeout time.Duration
	tool    string
}

func (h *sumHasher) Name() string { return h.tool }

func (h *sumHasher) Remote(ctx context.Context, p string) (string, error) {
	out, err := exec(ctx, h.s, h.timeout, h.tool+" "+shell.Quote(p))
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.tool, err)
	}
	f := strings.Fields(out)
	if len(f) == 0 {
		return "", fmt.Errorf("%s: empty output", h.tool)
	}
	return strings.ToLower(f[0]), nil
}

func (h *sumHasher) Local(data []byte) string {
	if h.tool == "md5sum" {
		s := md5.Sum(data)
		return hex.EncodeToString(s[:])
	}
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

// readback pulls the whole file as text and hashes it on the host.
type readback struct {
	s       transport.Session
	timeout time.Duration
	tool    string
}

func (h *readback) Name() string { return "readback-" + h.tool }

func (h *readback) Local(data []byte) string {
	s := sha256.Sum256(data)
	return hex.EncodeToString(s[:])
}

func (h *readback) Remote(ctx context.Context, p string) (string, error) {
	q := shell.Quote(p)
	var cmd string
	switch h.tool {
	case "base64":
		cmd = "base64 < " + q
	case "od":
		cmd = "od -An -v -tx1 " + q
	default:
		cmd = `hexdump -v -e '/1 "%02x"' ` + q
	}
	out, err := exec(ctx, h.s, h.timeout, cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", h.Name(), err)
	}
	text := strings.Join(strings.Fields(out), "")
	var data []byte
	if h.tool == "base64" {
		data, err = base64.StdEncoding.DecodeString(text)
	} else {
		data, err = hex.DecodeString(text)
	}
	if err != nil {
		return "", fmt.Errorf("%s: decode: %w", h.Name(), err)
	}
	return h.Local(data), nil
}
```

`internal/syncer/remote.go`:
```go
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
```

`internal/syncer/syncer.go`:
```go
package syncer

import (
	"context"
	"fmt"
	"path"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/shell"
	"devupdater/internal/transport"
	"devupdater/internal/upload"
)

const (
	NoteNotCompared = "not compared: device has no hash tool"
	NoteModeUnknown = "existing mode unreadable, used 0644"
)

type Device struct {
	S          transport.Session
	Hasher     Hasher // nil: device cannot hash or read back files
	Chain      *upload.Chain
	CmdTimeout time.Duration
}

type Options struct{ DryRun, Backup bool }

func (d *Device) run(ctx context.Context, cmd string) error {
	_, err := exec(ctx, d.S, d.CmdTimeout, cmd)
	return err
}

func (d *Device) SyncFile(ctx context.Context, f model.LocalFile, opt Options) (res model.FileResult) {
	start := time.Now()
	res.Remote = f.Remote
	defer func() { res.DurationMS = time.Since(start).Milliseconds() }()
	fail := func(step string, err error) model.FileResult {
		res.Status, res.Error = model.Failed, fmt.Sprintf("%s: %v", step, err)
		return res
	}

	existed, err := exists(ctx, d.S, d.CmdTimeout, f.Remote)
	if err != nil {
		return fail("check exists", err)
	}
	if existed {
		if d.Hasher == nil {
			res.Note = NoteNotCompared
		} else {
			rh, err := d.Hasher.Remote(ctx, f.Remote)
			if err != nil {
				return fail("remote hash", err)
			}
			if rh == d.Hasher.Local(f.Data) {
				res.Status = model.Unchanged
				return res
			}
		}
	}
	if opt.DryRun {
		res.Status = model.WouldUpdate
		if !existed {
			res.Status = model.WouldCreate
		}
		return res
	}

	mode := f.Mode
	if mode == "" {
		mode = "0644"
		if existed {
			if m, ok := readMode(ctx, d.S, d.CmdTimeout, f.Remote); ok {
				mode = m
			} else {
				res.Note = NoteModeUnknown
			}
		}
	}

	dst := shell.Quote(f.Remote)
	tmpPath := f.Remote + ".devupd.tmp"
	tmp := shell.Quote(tmpPath)
	cleanup := func() {
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), d.CmdTimeout)
		defer cancel()
		d.S.Exec(c, "rm -f "+tmp)
	}

	if err := d.run(ctx, "mkdir -p "+shell.Quote(path.Dir(f.Remote))); err != nil {
		return fail("mkdir", err)
	}
	method, err := d.Chain.Upload(ctx, f.Data, tmpPath)
	res.Method = method
	if err != nil {
		cleanup()
		return fail("upload", err)
	}
	if d.Hasher != nil {
		th, err := d.Hasher.Remote(ctx, tmpPath)
		if err != nil {
			cleanup()
			return fail("verify", err)
		}
		if th != d.Hasher.Local(f.Data) {
			cleanup()
			return fail("verify", fmt.Errorf("uploaded content hash mismatch"))
		}
	}
	if opt.Backup && existed {
		if err := d.run(ctx, "cp -p "+dst+" "+dst+".bak"); err != nil {
			cleanup()
			return fail("backup", err)
		}
	}
	if err := d.run(ctx, "chmod "+mode+" "+tmp); err != nil {
		cleanup()
		return fail("chmod", err)
	}
	if err := d.run(ctx, "mv -f "+tmp+" "+dst); err != nil {
		cleanup()
		return fail("rename", err)
	}
	res.Status = model.Created
	if existed {
		res.Status = model.Updated
	}
	return res
}
```

Note: `.bak` path is quoted as `'<remote>'.bak` — valid shell concatenation.

- [ ] **Step 4: Run, verify pass**

Run: `go test -race ./internal/syncer/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/syncer
git commit -m "feat: syncer with hash comparison, dry-run, backup and atomic rename"
```

---

### Task 11: Runner — worker pool, events, post_command

**Files:**
- Create: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

**Interfaces:**
- Consumes: `transport.Dial`, `transport.Options`, `probe.Probe`, `upload.Select`, `upload.NewChain`, `upload.FTPCreds`, `syncer.SelectHasher`, `syncer.Device`, `syncer.Options`, `workspace.Device`, `workspace.Settings`, `model.*`
- Produces:
  - `runner.Job{Devices []workspace.Device; Files []model.LocalFile; Settings workspace.Settings; DryRun bool; KnownHostsPath string; Dial DialFunc; Probe ProbeFunc}` (nil funcs → real `transport.Dial` / `probe.Probe`)
  - `runner.DialFunc func(ctx context.Context, d workspace.Device, opt transport.Options) (transport.Session, error)`
  - `runner.ProbeFunc func(ctx context.Context, s transport.Session, host string, timeout time.Duration) (probe.Caps, error)`
  - `runner.Event{Type, Host, Stage string; Done, Total int; File *model.FileResult; Error string; Run *model.RunResult}` (JSON tags snake_case; Type ∈ `device_state|file_result|run_done`; Stage ∈ `connecting|probing|syncing|post_command|done|failed`)
  - `runner.Run(ctx context.Context, job Job, emit func(Event)) model.RunResult` (emit may be nil; calls are serialized)

- [ ] **Step 1: Write failing tests**

`internal/runner/runner_test.go`:
```go
//go:build !windows

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

func localProbe(context.Context, transport.Session, string, time.Duration) (probe.Caps, error) {
	return probe.Caps{Tools: map[string]bool{"base64": true}}, nil
}

func file(remote, content string) model.LocalFile {
	return model.LocalFile{Remote: remote, Data: []byte(content)}
}

func baseJob(dir string) Job {
	s := workspace.DefaultSettings()
	s.CommandTimeoutSec = 5
	return Job{
		Settings: s,
		Files:    []model.LocalFile{file(filepath.Join(dir, "a.txt"), "A")},
		Probe:    localProbe,
		Dial: func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
			if d.Host == "down" {
				return nil, errors.New("connection refused")
			}
			return testutil.LocalShell{}, nil
		},
	}
}

func TestRunMixedDevices(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Devices = []workspace.Device{{Host: "down"}, {Host: "up"}}
	var mu sync.Mutex
	var types []string
	res := Run(context.Background(), job, func(e Event) {
		mu.Lock()
		types = append(types, e.Type)
		mu.Unlock()
	})
	down, up := res.Devices[0], res.Devices[1]
	if down.Error == "" || len(down.Files) != 1 || down.Files[0].Status != model.Failed {
		t.Fatalf("down: %+v", down)
	}
	if up.Error != "" || up.Files[0].Status != model.Created || up.Protocol != "local" || up.HashMethod != "readback-base64" {
		t.Fatalf("up: %+v", up)
	}
	if types[len(types)-1] != "run_done" || res.ID == "" || res.Finished.IsZero() {
		t.Fatalf("events %v res %+v", types, res)
	}
}

func TestPostCommandOnChange(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "post.ran")
	job := baseJob(dir)
	job.Devices = []workspace.Device{{Host: "up"}}
	job.Settings.PostCommand = "echo hi; touch " + marker

	res := Run(context.Background(), job, nil)
	if p := res.Devices[0].Post; p == nil || p.ExitCode != 0 || p.Output != "hi" {
		t.Fatalf("post: %+v", p)
	}
	os.Remove(marker)
	res = Run(context.Background(), job, nil) // nothing changes now
	if res.Devices[0].Post != nil {
		t.Fatalf("post must not run without changes: %+v", res.Devices[0].Post)
	}
	job.Settings.PostCommandPolicy = "always"
	res = Run(context.Background(), job, nil)
	if res.Devices[0].Post == nil {
		t.Fatal("policy always must run")
	}
	job.DryRun = true
	res = Run(context.Background(), job, nil)
	if res.Devices[0].Post != nil {
		t.Fatal("dry-run must not run post command")
	}
}

func TestParallelLimitAndPanicIsolation(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Settings.Parallel = 2
	job.DryRun = true
	var cur, peak int32
	job.Dial = func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
		n := atomic.AddInt32(&cur, 1)
		defer atomic.AddInt32(&cur, -1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		if d.Host == "boom" {
			panic("driver bug")
		}
		return testutil.LocalShell{}, nil
	}
	job.Devices = []workspace.Device{{Host: "1"}, {Host: "boom"}, {Host: "3"}, {Host: "4"}, {Host: "5"}}
	res := Run(context.Background(), job, nil)
	if peak > 2 {
		t.Fatalf("parallel limit exceeded: %d", peak)
	}
	if res.Devices[1].Error == "" {
		t.Fatal("panic not captured")
	}
	if res.Devices[4].Files[0].Status != model.WouldCreate {
		t.Fatalf("other devices affected: %+v", res.Devices[4])
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/runner/`
Expected: FAIL — `undefined: Run`.

- [ ] **Step 3: Implement**

`internal/runner/runner.go`:
```go
// Package runner syncs a manifest to many devices concurrently.
package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/syncer"
	"devupdater/internal/transport"
	"devupdater/internal/upload"
	"devupdater/internal/workspace"
)

type DialFunc func(ctx context.Context, d workspace.Device, opt transport.Options) (transport.Session, error)
type ProbeFunc func(ctx context.Context, s transport.Session, host string, timeout time.Duration) (probe.Caps, error)

type Job struct {
	Devices        []workspace.Device
	Files          []model.LocalFile
	Settings       workspace.Settings
	DryRun         bool
	KnownHostsPath string
	Dial           DialFunc
	Probe          ProbeFunc
}

type Event struct {
	Type  string            `json:"type"`
	Host  string            `json:"host,omitempty"`
	Stage string            `json:"stage,omitempty"`
	Done  int               `json:"done"`
	Total int               `json:"total"`
	File  *model.FileResult `json:"file,omitempty"`
	Error string            `json:"error,omitempty"`
	Run   *model.RunResult  `json:"run,omitempty"`
}

const maxPostOutput = 2000

func Run(ctx context.Context, job Job, emit func(Event)) model.RunResult {
	if job.Dial == nil {
		job.Dial = transport.Dial
	}
	if job.Probe == nil {
		job.Probe = probe.Probe
	}
	var emitMu sync.Mutex
	send := func(e Event) {
		if emit == nil {
			return
		}
		emitMu.Lock()
		defer emitMu.Unlock()
		emit(e)
	}

	start := time.Now()
	res := model.RunResult{
		ID: start.Format("20060102-150405"), Started: start,
		DryRun: job.DryRun, Parallel: job.Settings.Parallel,
		Devices: make([]model.DeviceResult, len(job.Devices)),
	}
	for _, f := range job.Files {
		res.Files = append(res.Files, f.Remote)
	}

	sem := make(chan struct{}, max(1, job.Settings.Parallel))
	var wg sync.WaitGroup
	for i, d := range job.Devices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			res.Devices[i] = runDevice(ctx, job, d, send)
		}()
	}
	wg.Wait()
	res.Finished = time.Now()
	send(Event{Type: "run_done", Run: &res})
	return res
}

func failAll(dr *model.DeviceResult, files []model.LocalFile, from int, msg string) {
	for _, f := range files[from:] {
		dr.Files = append(dr.Files, model.FileResult{Remote: f.Remote, Status: model.Failed, Error: msg})
	}
}

func runDevice(ctx context.Context, job Job, d workspace.Device, send func(Event)) (dr model.DeviceResult) {
	start := time.Now()
	dr.Host = d.Host
	total := len(job.Files)
	state := func(stage string, done int) {
		send(Event{Type: "device_state", Host: d.Host, Stage: stage, Done: done, Total: total, Error: dr.Error})
	}
	defer func() {
		if r := recover(); r != nil {
			dr.Error = fmt.Sprintf("internal error: %v", r)
			dr.Files = nil
			failAll(&dr, job.Files, 0, dr.Error)
		}
		dr.DurationMS = time.Since(start).Milliseconds()
		if dr.Failed() {
			state("failed", len(dr.Files))
		} else {
			state("done", len(dr.Files))
		}
	}()

	st := job.Settings
	cmdTimeout := time.Duration(st.CommandTimeoutSec) * time.Second
	opt := transport.Options{
		ConnectTimeout: time.Duration(st.ConnectTimeoutSec) * time.Second,
		CommandTimeout: cmdTimeout,
		StrictHostKey:  st.StrictHostKey,
		KnownHostsPath: job.KnownHostsPath,
	}

	state("connecting", 0)
	s, err := job.Dial(ctx, d, opt)
	if err != nil {
		dr.Error = "connect: " + err.Error()
		failAll(&dr, job.Files, 0, "device unreachable")
		return
	}
	defer s.Close()
	dr.Protocol = s.Protocol()

	state("probing", 0)
	caps, err := job.Probe(ctx, s, d.HostOnly(), opt.ConnectTimeout)
	if err != nil {
		dr.Error = "probe: " + err.Error()
		failAll(&dr, job.Files, 0, "probe failed")
		return
	}
	dr.Tools = caps.List()
	chain := upload.NewChain(upload.Select(s, caps,
		upload.FTPCreds{Host: d.HostOnly(), User: d.Username, Pass: d.Password}, cmdTimeout))
	h := syncer.SelectHasher(s, caps, cmdTimeout)
	dr.HashMethod = "none"
	if h != nil {
		dr.HashMethod = h.Name()
	}
	dev := &syncer.Device{S: s, Hasher: h, Chain: chain, CmdTimeout: cmdTimeout}

	changed := false
	for i, f := range job.Files {
		if ctx.Err() != nil {
			failAll(&dr, job.Files, i, "cancelled")
			break
		}
		state("syncing", i)
		fr := dev.SyncFile(ctx, f, syncer.Options{DryRun: job.DryRun, Backup: st.Backup})
		dr.Files = append(dr.Files, fr)
		changed = changed || fr.Status == model.Created || fr.Status == model.Updated
		send(Event{Type: "file_result", Host: d.Host, Done: i + 1, Total: total, File: &fr})
	}
	dr.UploadMethod = chain.Primary()

	runPost := !job.DryRun && st.PostCommand != "" && ctx.Err() == nil &&
		(st.PostCommandPolicy == "always" || (st.PostCommandPolicy == "on_change" && changed))
	if runPost {
		state("post_command", total)
		c, cancel := context.WithTimeout(ctx, cmdTimeout)
		out, code, err := s.Exec(c, st.PostCommand)
		cancel()
		if len(out) > maxPostOutput {
			out = out[:maxPostOutput] + "…"
		}
		dr.Post = &model.PostResult{Command: st.PostCommand, ExitCode: code, Output: out}
		if err != nil {
			dr.Post.Error = err.Error()
		}
	}
	return
}
```

- [ ] **Step 4: Run, verify pass**

Run: `go test -race ./internal/runner/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/runner
git commit -m "feat: runner with worker pool, per-device isolation, events, post command"
```

---

### Task 12: Reports — JSON, HTML, console

**Files:**
- Create: `internal/report/report.go`, `internal/report/html.go`, `internal/report/report.html.tmpl`, `internal/report/console.go`
- Test: `internal/report/report_test.go`

**Interfaces:**
- Consumes: `model.RunResult`, `model.DeviceResult`, `model.Status`
- Produces:
  - `report.Counts{Devices, DevicesFailed int; ByStatus map[model.Status]int}`; `report.Summarize(r model.RunResult) Counts`
  - `report.Save(dir string, r model.RunResult) (htmlPath string, err error)` — writes `<dir>/<ID>.json` and `<dir>/<ID>.html`, creating `dir`
  - `report.Load(path string) (model.RunResult, error)`
  - `report.FailedHosts(r model.RunResult) []string`
  - `report.WriteHTML(w io.Writer, r model.RunResult) error`
  - `report.PrintConsole(w io.Writer, r model.RunResult)`

Report labels are Turkish (operator-facing); status codes stay as in the model.

- [ ] **Step 1: Write failing tests**

`internal/report/report_test.go`:
```go
package report

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devupdater/internal/model"
)

func sample() model.RunResult {
	return model.RunResult{
		ID: "20260930-120000", Started: time.Now(), Finished: time.Now(), Parallel: 10,
		Files: []string{"/opt/a", "/etc/b"},
		Devices: []model.DeviceResult{
			{Host: "10.0.0.1", Protocol: "ssh", UploadMethod: "sftp", HashMethod: "sha256sum",
				Files: []model.FileResult{{Remote: "/opt/a", Status: model.Updated}, {Remote: "/etc/b", Status: model.Unchanged}}},
			{Host: "10.0.0.2", Error: "connect: refused <script>",
				Files: []model.FileResult{{Remote: "/opt/a", Status: model.Failed, Error: "device unreachable"}, {Remote: "/etc/b", Status: model.Failed}}},
		},
	}
}

func TestSummarizeAndFailedHosts(t *testing.T) {
	c := Summarize(sample())
	if c.Devices != 2 || c.DevicesFailed != 1 || c.ByStatus[model.Failed] != 2 || c.ByStatus[model.Updated] != 1 {
		t.Fatalf("%+v", c)
	}
	if h := FailedHosts(sample()); len(h) != 1 || h[0] != "10.0.0.2" {
		t.Fatalf("%v", h)
	}
}

func TestSaveLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reports")
	htmlPath, err := Save(dir, sample())
	if err != nil {
		t.Fatal(err)
	}
	html, _ := os.ReadFile(htmlPath)
	if !strings.Contains(string(html), "10.0.0.2") || strings.Contains(string(html), "<script>") {
		t.Fatal("html missing host or not escaped")
	}
	r, err := Load(filepath.Join(dir, "20260930-120000.json"))
	if err != nil || len(r.Devices) != 2 || r.Devices[0].Files[0].Status != model.Updated {
		t.Fatalf("%+v %v", r, err)
	}
}

func TestConsole(t *testing.T) {
	var b bytes.Buffer
	PrintConsole(&b, sample())
	out := b.String()
	for _, want := range []string{"UPDATED", "FAILED", "10.0.0.2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("console missing %q:\n%s", want, out)
		}
	}
}
```

Password leakage is impossible by construction: `model` types have no password field. Task 13's CLI test asserts it end-to-end.

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/report/`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Implement**

`internal/report/report.go`:
```go
// Package report renders run results as JSON, HTML and console text.
package report

import (
	"encoding/json"
	"os"
	"path/filepath"

	"devupdater/internal/model"
)

type Counts struct {
	Devices       int
	DevicesFailed int
	ByStatus      map[model.Status]int
}

func Summarize(r model.RunResult) Counts {
	c := Counts{Devices: len(r.Devices), ByStatus: map[model.Status]int{}}
	for _, d := range r.Devices {
		if d.Failed() {
			c.DevicesFailed++
		}
		for _, f := range d.Files {
			c.ByStatus[f.Status]++
		}
	}
	return c
}

func FailedHosts(r model.RunResult) []string {
	var out []string
	for _, d := range r.Devices {
		if d.Failed() {
			out = append(out, d.Host)
		}
	}
	return out
}

func Save(dir string, r model.RunResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, r.ID+".json"), b, 0o644); err != nil {
		return "", err
	}
	htmlPath := filepath.Join(dir, r.ID+".html")
	f, err := os.Create(htmlPath)
	if err != nil {
		return "", err
	}
	if err := WriteHTML(f, r); err != nil {
		f.Close()
		return "", err
	}
	return htmlPath, f.Close()
}

func Load(path string) (model.RunResult, error) {
	var r model.RunResult
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}
```

`internal/report/html.go`:
```go
package report

import (
	_ "embed"
	"html/template"
	"io"

	"devupdater/internal/model"
)

//go:embed report.html.tmpl
var htmlTmpl string

var tmpl = template.Must(template.New("report").Parse(htmlTmpl))

var statusOrder = []model.Status{model.Created, model.Updated, model.Unchanged, model.WouldCreate, model.WouldUpdate, model.Failed}

func WriteHTML(w io.Writer, r model.RunResult) error {
	return tmpl.Execute(w, map[string]any{
		"Run":         r,
		"Counts":      Summarize(r),
		"StatusOrder": statusOrder,
		"Duration":    r.Finished.Sub(r.Started).Round(1e9).String(),
	})
}
```

`internal/report/report.html.tmpl`:
```html
<!doctype html>
<html lang="tr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>DeviceFileUpdater Raporu {{.Run.ID}}</title>
<style>
:root{--bg:#fafafa;--fg:#1a1a1a;--muted:#666;--line:#e3e3e3;--ok:#1f7a3a;--chg:#1d5fb8;--bad:#b3261e;--card:#fff}
@media (prefers-color-scheme:dark){:root{--bg:#121212;--fg:#eee;--muted:#9a9a9a;--line:#2c2c2c;--ok:#5cc07a;--chg:#6fa8f5;--bad:#f2766b;--card:#1b1b1b}}
body{font:14px/1.5 system-ui,sans-serif;background:var(--bg);color:var(--fg);margin:0;padding:24px}
h1{font-size:20px;margin:0 0 4px}.muted{color:var(--muted)}
.cards{display:flex;flex-wrap:wrap;gap:12px;margin:16px 0}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:10px 14px;min-width:110px}
.card b{display:block;font-size:22px}
table{border-collapse:collapse;width:100%;background:var(--card);margin:8px 0 24px}
th,td{border-bottom:1px solid var(--line);padding:6px 8px;text-align:left;vertical-align:top}
th{font-weight:600;color:var(--muted)}
.CREATED,.UPDATED{color:var(--chg)}.UNCHANGED{color:var(--ok)}.FAILED{color:var(--bad);font-weight:600}
.dev{margin-top:20px}.err{color:var(--bad)}code{font-size:12px}
</style>
</head>
<body>
<h1>Dosya güncelleme raporu {{if .Run.DryRun}}(ön izleme / dry-run){{end}}</h1>
<div class="muted">Çalıştırma {{.Run.ID}} · süre {{.Duration}} · paralel {{.Run.Parallel}}</div>

<div class="cards">
  <div class="card"><b>{{.Counts.Devices}}</b>cihaz</div>
  <div class="card"><b class="FAILED">{{.Counts.DevicesFailed}}</b>hatalı cihaz</div>
  {{range $s := .StatusOrder}}{{with index $.Counts.ByStatus $s}}<div class="card"><b class="{{$s}}">{{.}}</b>{{$s}}</div>{{end}}{{end}}
</div>

<h2>Dosyalar</h2>
<ul>{{range .Run.Files}}<li><code>{{.}}</code></li>{{end}}</ul>

{{range .Run.Devices}}
<div class="dev">
  <h3>{{.Host}} <span class="muted">{{.Protocol}} · yükleme {{.UploadMethod}} · hash {{.HashMethod}} · {{.DurationMS}} ms</span></h3>
  {{if .Error}}<div class="err">{{.Error}}</div>{{end}}
  <table>
    <tr><th>Dosya</th><th>Durum</th><th>Yöntem</th><th>ms</th><th>Hata / not</th></tr>
    {{range .Files}}
    <tr><td><code>{{.Remote}}</code></td><td class="{{.Status}}">{{.Status}}</td><td>{{.Method}}</td><td>{{.DurationMS}}</td><td>{{.Error}}{{if and .Error .Note}} · {{end}}{{.Note}}</td></tr>
    {{end}}
  </table>
  {{with .Post}}<div>Sonrası komut <code>{{.Command}}</code> → çıkış {{.ExitCode}} {{if .Error}}<span class="err">{{.Error}}</span>{{end}}<pre>{{.Output}}</pre></div>{{end}}
</div>
{{end}}
</body>
</html>
```


`internal/report/console.go`:
```go
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
```

- [ ] **Step 4: Run, verify pass**

Run: `go test ./internal/report/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/report
git commit -m "feat: html/json/console run reports"
```

---

### Task 13: CLI `run` command

**Files:**
- Create: `internal/cli/cli.go`, `cmd/devupdater/main.go`
- Test: `internal/cli/cli_test.go`

**Interfaces:**
- Consumes: `workspace.*`, `runner.Run`, `runner.Job`, `runner.Event`, `report.Save`, `report.Load`, `report.FailedHosts`, `report.PrintConsole`
- Produces: `cli.Main(args []string, stdout, stderr io.Writer) int`

Usage:
```
devupdater run [-workspace DIR] [-dry-run] [-parallel N] [-only-failed REPORT_ID]
```
`-parallel 0` (default) uses settings. `-only-failed` reads `reports/<ID>.json` and keeps only devices that failed there. Plan 2 adds `ui` and makes it the default when no subcommand is given; until then no subcommand prints usage and returns 2.

- [ ] **Step 1: Write failing tests**

`internal/cli/cli_test.go`:
```go
package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devupdater/internal/workspace"
)

func setupWS(t *testing.T) string {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "files"), 0o755)
	os.WriteFile(filepath.Join(dir, "files", "a.txt"), []byte("A"), 0o644)
	workspace.SaveDevices(filepath.Join(dir, "devices.csv"),
		[]workspace.Device{{Host: "127.0.0.1:1", Username: "root", Password: "SuperSecret123"}})
	workspace.SaveManifest(filepath.Join(dir, "manifest.csv"),
		[]workspace.Entry{{LocalPath: "files/a.txt", RemotePath: "/tmp/a.txt"}})
	s := workspace.DefaultSettings()
	s.ConnectTimeoutSec = 1
	workspace.SaveSettings(filepath.Join(dir, "settings.json"), s)
	return dir
}

func TestUsageAndConfigErrors(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Main(nil, &out, &errb); code != 2 {
		t.Fatalf("no args: %d", code)
	}
	if code := Main([]string{"run", "-workspace", t.TempDir()}, &out, &errb); code != 2 {
		t.Fatalf("empty workspace: %d", code)
	}
	dir := setupWS(t)
	if code := Main([]string{"run", "-workspace", dir, "-only-failed", "nope"}, &out, &errb); code != 2 {
		t.Fatalf("unknown report: %d", code)
	}
}

func TestRunUnreachableWritesReportWithoutPassword(t *testing.T) {
	dir := setupWS(t)
	var out, errb bytes.Buffer
	code := Main([]string{"run", "-workspace", dir}, &out, &errb)
	if code != 1 {
		t.Fatalf("exit %d, stderr %s", code, errb.String())
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "reports"))
	if len(entries) != 2 {
		t.Fatalf("expected json+html, got %d", len(entries))
	}
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, "reports", e.Name()))
		if strings.Contains(string(b), "SuperSecret123") {
			t.Fatalf("password leaked into %s", e.Name())
		}
	}
	all := out.String() + errb.String()
	if strings.Contains(all, "SuperSecret123") || !strings.Contains(all, "FAILED") {
		t.Fatalf("console: %s", all)
	}
	// Retry only failed devices from that report.
	id := strings.TrimSuffix(entries[0].Name(), filepath.Ext(entries[0].Name()))
	if code := Main([]string{"run", "-workspace", dir, "-only-failed", id}, &out, &errb); code != 1 {
		t.Fatalf("only-failed exit %d", code)
	}
}
```

- [ ] **Step 2: Run, verify failure**

Run: `go test ./internal/cli/`
Expected: FAIL — `undefined: Main`.

- [ ] **Step 3: Implement**

`internal/cli/cli.go`:
```go
// Package cli implements the devupdater command line.
package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"

	"devupdater/internal/model"
	"devupdater/internal/report"
	"devupdater/internal/runner"
	"devupdater/internal/workspace"
)

const usage = `usage:
  devupdater run [-workspace DIR] [-dry-run] [-parallel N] [-only-failed REPORT_ID]
`

func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprint(stderr, usage)
		return 2
	}
	return runCmd(args[1:], stdout, stderr)
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
	ws := workspace.Workspace{Dir: *dir}
	cfgErr := func(err error) int { fmt.Fprintln(stderr, "error:", err); return 2 }

	settings, err := workspace.LoadSettings(ws.SettingsPath())
	if err != nil {
		return cfgErr(err)
	}
	if *parallel > 0 {
		settings.Parallel = *parallel
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	done := 0
	res := runner.Run(ctx, runner.Job{
		Devices: devices, Files: files, Settings: settings,
		DryRun: *dryRun, KnownHostsPath: ws.KnownHostsPath(),
	}, func(e runner.Event) {
		if e.Type == "device_state" && (e.Stage == "done" || e.Stage == "failed") {
			done++
			fmt.Fprintf(stdout, "[%d/%d] %s %s\n", done, len(devices), e.Host, e.Stage)
		}
	})

	htmlPath, err := report.Save(ws.ReportsDir(), res)
	if err != nil {
		fmt.Fprintln(stderr, "error: writing report:", err)
	}
	report.PrintConsole(stdout, res)
	if htmlPath != "" {
		fmt.Fprintln(stdout, "report:", htmlPath)
	}
	return exitCode(res)
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
```

Note: two runs in the same second share a report ID; the test's second run overwrites the first report, which is acceptable.

`cmd/devupdater/main.go`:
```go
package main

import (
	"os"

	"devupdater/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
```

- [ ] **Step 4: Run, verify pass**

Run: `go test ./internal/cli/ && go build ./cmd/devupdater`
Expected: PASS, binary builds.

- [ ] **Step 5: Commit**

```bash
git add internal/cli cmd
git commit -m "feat: devupdater run CLI with reports and exit codes"
```

---

### Task 14: Docker device matrix integration tests + Makefile

**Files:**
- Create: `test/integration/docker-compose.yml`, `test/integration/devices/ssh-full/Dockerfile`, `test/integration/devices/ssh-dropbear/Dockerfile`, `test/integration/devices/telnet-ftp/Dockerfile`, `test/integration/devices/telnet-min/Dockerfile`, `test/integration/integration_test.go`, `Makefile`

**Interfaces:**
- Consumes: `transport.Dial`, `runner.Run`, `runner.Job`, `workspace.Device`, `workspace.DefaultSettings`, `model.*`

Device matrix (all users `dev` / `devpass`, writable `/data`):

| service | protocol | expected upload | expected hash |
|---|---|---|---|
| `dev-a` | ssh (openssh) | `sftp` | `sha256sum` |
| `dev-b` | ssh (dropbear, no scp/sftp) | `shell-base64` | `sha256sum` |
| `dev-c` | telnet + busybox ftpd | `ftp` | `sha256sum` |
| `dev-d` | telnet, base64/md5sum/sha256sum/od/hexdump/stat removed | `shell-printf` | `none` |

Tests run inside a `tester` container on the compose network so each device has its own IP and default ports (22/23/21).

- [ ] **Step 1: Device images**

`test/integration/devices/ssh-full/Dockerfile`:
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache openssh openssh-sftp-server && ssh-keygen -A \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data \
 && sed -i 's/^#\?PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
CMD ["/usr/sbin/sshd", "-D", "-e"]
```

`test/integration/devices/ssh-dropbear/Dockerfile`:
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache dropbear && mkdir -p /etc/dropbear \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data
CMD ["dropbear", "-F", "-E", "-R", "-p", "22"]
```

`test/integration/devices/telnet-ftp/Dockerfile`:
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache busybox-extras \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data
CMD ["sh", "-c", "telnetd -l /bin/login && exec tcpsvd -vE 0.0.0.0 21 ftpd -w /"]
```

`test/integration/devices/telnet-min/Dockerfile`:
```dockerfile
FROM alpine:3.20
RUN apk add --no-cache busybox-extras \
 && adduser -D -s /bin/sh dev && echo 'dev:devpass' | chpasswd \
 && mkdir -p /data && chown dev /data \
 && for t in base64 md5sum sha256sum od hexdump stat scp; do p=$(which $t) && rm -f "$p"; done; true
CMD ["telnetd", "-F", "-l", "/bin/login"]
```

`test/integration/docker-compose.yml`:
```yaml
services:
  dev-a: { build: ./devices/ssh-full }
  dev-b: { build: ./devices/ssh-dropbear }
  dev-c: { build: ./devices/telnet-ftp }
  dev-d: { build: ./devices/telnet-min }
  tester:
    image: golang:1.25-alpine
    working_dir: /src
    volumes: ["../..:/src"]
    environment: { CGO_ENABLED: "0" }
    command: go test -tags integration -count=1 -v ./test/integration/...
    depends_on: [dev-a, dev-b, dev-c, dev-d]
```

- [ ] **Step 2: Write the integration test**

`test/integration/integration_test.go`:
```go
//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/runner"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

type devCase struct {
	host, proto, upload, hash string
}

var cases = []devCase{
	{"dev-a", "ssh", "sftp", "sha256sum"},
	{"dev-b", "ssh", "shell-base64", "sha256sum"},
	{"dev-c", "telnet", "ftp", "sha256sum"},
	{"dev-d", "telnet", "shell-printf", "none"},
}

func dev(h string) workspace.Device { return workspace.Device{Host: h, Username: "dev", Password: "devpass"} }

func opts() transport.Options {
	return transport.Options{ConnectTimeout: 5 * time.Second, CommandTimeout: 30 * time.Second}
}

func waitReady(t *testing.T, h string) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		s, err := transport.Dial(context.Background(), dev(h), opts())
		if err == nil {
			s.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s not ready: %v", h, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func exec(t *testing.T, h, cmd string) string {
	s, err := transport.Dial(context.Background(), dev(h), opts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	out, _, err := s.Exec(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func lf(remote string, data []byte, mode string) model.LocalFile {
	return model.LocalFile{Remote: remote, Data: data, Mode: mode}
}

func run(h string, files []model.LocalFile, dry bool, post string) model.DeviceResult {
	s := workspace.DefaultSettings()
	s.PostCommand = post
	return runner.Run(context.Background(), runner.Job{
		Devices: []workspace.Device{dev(h)}, Files: files, Settings: s, DryRun: dry,
	}, nil).Devices[0]
}

func statuses(d model.DeviceResult) []model.Status {
	var out []model.Status
	for _, f := range d.Files {
		out = append(out, f.Status)
	}
	return out
}

func TestDeviceMatrix(t *testing.T) {
	bin := make([]byte, 5000)
	rand.Read(bin)
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			waitReady(t, c.host)
			files := []model.LocalFile{
				lf("/data/app/bin.dat", bin, "0755"),
				lf("/data/conf.txt", []byte("version=1\n"), ""),
			}

			d := run(c.host, files, true, "")
			if d.Error != "" || statuses(d)[0] != model.WouldCreate || statuses(d)[1] != model.WouldCreate {
				t.Fatalf("dry-run: %+v", d)
			}
			if d.Protocol != c.proto || d.HashMethod != c.hash {
				t.Fatalf("protocol/hash: %s/%s", d.Protocol, d.HashMethod)
			}

			d = run(c.host, files, false, "")
			if statuses(d)[0] != model.Created || statuses(d)[1] != model.Created || d.UploadMethod != c.upload {
				t.Fatalf("create: %+v", d)
			}
			if got := strings.TrimSpace(exec(t, c.host, "wc -c < /data/app/bin.dat")); got != "5000" {
				t.Fatalf("bin size %s", got)
			}
			if got := exec(t, c.host, "ls -ln /data/app/bin.dat"); !strings.HasPrefix(got, "-rwxr-xr-x") {
				t.Fatalf("mode: %s", got)
			}

			d = run(c.host, files, false, "")
			want := model.Unchanged
			if c.hash == "none" {
				want = model.Updated // cannot compare: always rewritten
			}
			if statuses(d)[0] != want || statuses(d)[1] != want {
				t.Fatalf("second run: %+v", d)
			}

			files[1] = lf("/data/conf.txt", []byte("version=2\n"), "")
			d = run(c.host, files, false, "cat /data/conf.txt")
			if statuses(d)[1] != model.Updated || d.Post == nil || d.Post.Output != "version=2" {
				t.Fatalf("update: %+v post %+v", d, d.Post)
			}
			if got := exec(t, c.host, "cat /data/conf.txt.bak"); got != "version=1" {
				t.Fatalf("backup: %q", got)
			}
			if got := exec(t, c.host, "ls /data/*.devupd.tmp /data/app/*.devupd.tmp 2>/dev/null | wc -l"); strings.TrimSpace(got) != "0" {
				t.Fatalf("tmp files left: %s", got)
			}

			d = run(c.host, []model.LocalFile{lf("/root/forbidden.txt", []byte("x"), "")}, false, "")
			if statuses(d)[0] != model.Failed {
				t.Fatalf("forbidden path: %+v", d)
			}
		})
	}
}

func TestWrongPassword(t *testing.T) {
	waitReady(t, "dev-a")
	d := runner.Run(context.Background(), runner.Job{
		Devices:  []workspace.Device{{Host: "dev-a", Username: "dev", Password: "wrong"}},
		Files:    []model.LocalFile{lf("/data/x", []byte("x"), "")},
		Settings: workspace.DefaultSettings(),
	}, nil).Devices[0]
	if !strings.Contains(d.Error, "authentication failed") {
		t.Fatalf("%+v", d)
	}
}
```

- [ ] **Step 3: Makefile**

`Makefile`:
```make
TARGETS := linux/amd64 linux/arm64 linux/arm windows/amd64 windows/arm64 darwin/amd64 darwin/arm64

.PHONY: test integration release

test:
	go vet ./...
	go test -race ./...

integration:
	docker compose -f test/integration/docker-compose.yml up --build --abort-on-container-exit --exit-code-from tester
	docker compose -f test/integration/docker-compose.yml down

release:
	rm -rf dist && mkdir -p dist
	for t in $(TARGETS); do \
	  os=$${t%/*}; arch=$${t#*/}; ext=; [ $$os = windows ] && ext=.exe; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "-s -w" \
	    -o dist/devupdater-$$os-$$arch$$ext ./cmd/devupdater || exit 1; \
	done
```

- [ ] **Step 4: Run everything**

Run: `make test`
Expected: PASS.

Run: `make integration`
Expected: `tester` exits 0; all four `TestDeviceMatrix/dev-*` subtests and `TestWrongPassword` PASS. If a device image behaves differently from the table (e.g. busybox login prompt text, dropbear exec), fix the transport code — not the test expectations — unless the spec's order rules say otherwise.

Run: `make release && ls dist`
Expected: 7 binaries, windows ones with `.exe`.

- [ ] **Step 5: Commit**

```bash
git add test Makefile
git commit -m "test: docker device matrix integration suite and release build"
```

---

## Out of this plan (Plan 2)

`devupdater ui` (default command), REST + SSE API, React wizard (workspace, devices + connection test, files, settings, dry-run preview, apply, report, history). Plan 2 consumes `workspace`, `runner.Run`/`runner.Event`, `report.Save/Load/FailedHosts`, `probe.Probe`, `transport.Dial` unchanged.
