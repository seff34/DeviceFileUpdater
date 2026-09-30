package report

import (
	"bytes"
	"html/template"
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

// A template failure must not leave a .json without its .html (or a partial .html).
func TestSaveWritesNothingOnTemplateError(t *testing.T) {
	old := tmpl
	tmpl = template.Must(template.New("report").Parse(`partial{{template "missing"}}`))
	t.Cleanup(func() { tmpl = old })
	dir := t.TempDir()
	if _, err := Save(dir, sample()); err == nil {
		t.Fatal("expected template error")
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Fatalf("files written: %v", ents)
	}
}

// Device-supplied text must not inject terminal escapes into the console.
func TestConsoleEscapesControlChars(t *testing.T) {
	r := sample()
	r.Devices[1].Error = "boom\x1b[2J\x1b]0;pwned\x07\r\nnext\tcol"
	r.Devices = append(r.Devices, model.DeviceResult{Host: "10.0.0.3",
		Files: []model.FileResult{{Remote: "/x", Status: model.Failed, Error: "bad\x1b[31mred"}},
		Post:  &model.PostResult{ExitCode: 1, Error: "e\x9b1m"}})
	var b bytes.Buffer
	PrintConsole(&b, r)
	out := b.String()
	if strings.ContainsAny(out, "\x1b\x07\r\x9b") || strings.Contains(out, "\u009b") {
		t.Fatalf("raw control characters in console output: %q", out)
	}
	for _, want := range []string{`boom\x1b[2J\x1b]0;pwned\a\r\nnext` + "\tcol", `bad\x1b[31mred`, `e\x9b1m`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}
