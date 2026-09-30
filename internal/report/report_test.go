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

func TestSaveRejectsUnsafeID(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "reports")
	for _, id := range []string{"", "..", "../evil", "a/b", `a\b`, "x..y/../z"} {
		r := sample()
		r.ID = id
		if _, err := Save(dir, r); err == nil {
			t.Errorf("id %q: expected error", id)
		}
	}
	if _, err := os.Stat(filepath.Join(base, "evil.json")); err == nil {
		t.Fatal("wrote outside dir")
	}
	if es, _ := os.ReadDir(dir); len(es) != 0 {
		t.Fatalf("files written for unsafe ids: %v", es)
	}
}

func TestZeroTimesAndDryRun(t *testing.T) {
	var b bytes.Buffer
	if err := WriteHTML(&b, model.RunResult{ID: "x", DryRun: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "dry-run") {
		t.Fatal("dry-run marker missing")
	}
}
