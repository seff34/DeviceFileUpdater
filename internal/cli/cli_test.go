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
	t.Helper()
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
	if code := Main([]string{"bogus"}, &out, &errb); code != 2 {
		t.Fatalf("unknown subcommand: %d", code)
	}
	if code := Main([]string{"run", "-workspace", t.TempDir()}, &out, &errb); code != 2 {
		t.Fatalf("empty workspace: %d", code)
	}
	dir := setupWS(t)
	if code := Main([]string{"run", "-workspace", dir, "-only-failed", "nope"}, &out, &errb); code != 2 {
		t.Fatalf("unknown report: %d", code)
	}
	if code := Main([]string{"run", "-workspace", dir, "-parallel", "500"}, &out, &errb); code != 2 {
		t.Fatalf("parallel out of range: %d", code)
	}
}

func TestOnlyFailedRejectsUnsafeID(t *testing.T) {
	dir := setupWS(t)
	// A decoy report outside reports/ that a traversal would reach.
	os.WriteFile(filepath.Join(dir, "evil.json"), []byte(`{"id":"evil","devices":[]}`), 0o644)
	for _, id := range []string{"../evil", "a/b", `a\b`, "..", "x..y"} {
		var out, errb bytes.Buffer
		if code := Main([]string{"run", "-workspace", dir, "-only-failed", id}, &out, &errb); code != 2 {
			t.Fatalf("id %q: exit %d", id, code)
		}
		if !strings.Contains(errb.String(), "unsafe") {
			t.Fatalf("id %q: stderr %q", id, errb.String())
		}
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
	var jsonName string
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(dir, "reports", e.Name()))
		if strings.Contains(string(b), "SuperSecret123") {
			t.Fatalf("password leaked into %s", e.Name())
		}
		if filepath.Ext(e.Name()) == ".json" {
			jsonName = e.Name()
		}
	}
	all := out.String() + errb.String()
	if strings.Contains(all, "SuperSecret123") || !strings.Contains(all, "FAILED") {
		t.Fatalf("console: %s", all)
	}
	// Retry only failed devices from that report.
	id := strings.TrimSuffix(jsonName, ".json")
	out.Reset()
	errb.Reset()
	if code := Main([]string{"run", "-workspace", dir, "-only-failed", id}, &out, &errb); code != 1 {
		t.Fatalf("only-failed exit %d", code)
	}
	if strings.Contains(out.String()+errb.String(), "SuperSecret123") {
		t.Fatal("password leaked on retry")
	}
}
