package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDevices(t *testing.T) {
	ok := []Device{{Host: "10.0.0.1", Username: "root", Password: "x"}, {Host: "10.0.0.2:2222", Username: "u"}}
	if err := ValidateDevices(ok); err != nil {
		t.Fatalf("valid devices rejected: %v", err)
	}
	cases := map[string][]Device{
		"required":  {{Host: "", Username: "root"}},
		"required2": {{Host: "10.0.0.1", Username: " "}},
		"duplicate": {{Host: "10.0.0.1", Username: "a"}, {Host: "10.0.0.1", Username: "b"}},
		"space":     {{Host: "10.0.0.1 x", Username: "a"}},
	}
	for name, ds := range cases {
		if err := ValidateDevices(ds); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	if err := ValidateDevices(nil); err != nil {
		t.Errorf("empty list must be valid (draft), got %v", err)
	}
}

func TestLoadManifestDraft(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.csv")
	es, err := LoadManifestDraft(p)
	if err != nil || es != nil {
		t.Fatalf("missing file: got %v, %v", es, err)
	}
	os.WriteFile(p, []byte("local_path,remote_path,mode\n"), 0o644)
	es, err = LoadManifestDraft(p)
	if err != nil || len(es) != 0 {
		t.Fatalf("header only: got %v, %v", es, err)
	}
	// LoadManifest (strict) still rejects the empty manifest
	if _, err := LoadManifest(p); err == nil {
		t.Fatal("LoadManifest must reject an empty manifest")
	}
	// draft accepts rows that strict validation would reject
	os.WriteFile(p, []byte("\xef\xbb\xbflocal_path,remote_path,mode\nfiles/a,relative/path,\n"), 0o644)
	es, err = LoadManifestDraft(p)
	if err != nil || len(es) != 1 || es[0].RemotePath != "relative/path" {
		t.Fatalf("draft rows: got %v, %v", es, err)
	}
}

func TestSaveManifestAndSettingsAtomic(t *testing.T) {
	dir := t.TempDir()
	mp := filepath.Join(dir, "manifest.csv")
	if err := SaveManifest(mp, []Entry{{LocalPath: "files/a", RemotePath: "/opt/a", Mode: "0755"}}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(mp)
	if !strings.Contains(string(b), "\ufefflocal_path;remote_path;mode\nfiles/a;/opt/a;0755") {
		t.Fatalf("manifest content: %q", b)
	}
	sp := filepath.Join(dir, "settings.json")
	if err := SaveSettings(sp, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("temp file left behind: %s", e.Name())
		}
	}
}
