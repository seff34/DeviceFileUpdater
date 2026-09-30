package workspace

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadManifestWithBOM(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "manifest.csv")
	// Write file with UTF-8 BOM prefix
	os.WriteFile(p, []byte("\xef\xbb\xbflocal_path,remote_path,mode\nfiles/a.txt,/tmp/a.txt,644\n"), 0o644)
	es, err := LoadManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(es) != 1 || es[0].LocalPath != "files/a.txt" {
		t.Fatalf("got %+v", es)
	}
}

// Control bytes in remote_path would corrupt the line-based shell protocol.
func TestRemotePathControlBytesRejected(t *testing.T) {
	for _, p := range []string{"/a\nb", "/a\rb", "/a\x00b", "/a\tb", "/a\x1b[2Jb", "/a\x7fb"} {
		err := ValidateEntries([]Entry{{LocalPath: "a", RemotePath: p}})
		if err == nil || !strings.Contains(err.Error(), "control character") {
			t.Errorf("%q: err = %v", p, err)
		}
	}
	if err := ValidateEntries([]Entry{{LocalPath: "a", RemotePath: "/opt/app dir/ü.conf"}}); err != nil {
		t.Fatal(err)
	}
}
