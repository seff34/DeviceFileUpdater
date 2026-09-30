package web

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

type wsResp struct {
	Current string   `json:"current"`
	Recent  []string `json:"recent"`
}

func TestCreateWorkspaceScaffolds(t *testing.T) {
	_, ts, c := newTestServer(t)
	dir := filepath.Join(t.TempDir(), "yeni-alan")
	var got wsResp
	if code := doJSON(t, c, "POST", ts.URL+"/api/workspace", map[string]any{"path": dir, "create": true}, &got); code != 200 {
		t.Fatalf("status %d", code)
	}
	if got.Current != dir || len(got.Recent) != 1 || got.Recent[0] != dir {
		t.Fatalf("resp: %+v", got)
	}
	for _, f := range []string{"devices.csv", "manifest.csv", "settings.json", "files", "reports"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	st, _ := os.Stat(filepath.Join(dir, "devices.csv"))
	if st.Mode().Perm() != 0o600 {
		t.Errorf("devices.csv perm %v", st.Mode().Perm())
	}
}

func TestCreateKeepsExistingFiles(t *testing.T) {
	_, ts, c := newTestServer(t)
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "devices.csv"), []byte("ip,username,password\n10.0.0.1,root,x\n"), 0o600)
	if code := doJSON(t, c, "POST", ts.URL+"/api/workspace", map[string]any{"path": dir, "create": true}, nil); code != 200 {
		t.Fatalf("status %d", code)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "devices.csv"))
	if string(b) != "ip,username,password\n10.0.0.1,root,x\n" {
		t.Fatalf("existing devices.csv overwritten: %q", b)
	}
}

func TestOpenWorkspaceErrors(t *testing.T) {
	_, ts, c := newTestServer(t)
	var e struct{ Error string }
	if code := doJSON(t, c, "POST", ts.URL+"/api/workspace", map[string]any{"path": "relative/dir"}, &e); code != http.StatusUnprocessableEntity {
		t.Fatalf("relative: %d", code)
	}
	if code := doJSON(t, c, "POST", ts.URL+"/api/workspace", map[string]any{"path": filepath.Join(t.TempDir(), "yok")}, &e); code != http.StatusNotFound {
		t.Fatalf("missing: %d", code)
	}
	var got wsResp
	doJSON(t, c, "GET", ts.URL+"/api/workspace", nil, &got)
	if got.Current != "" || got.Recent == nil {
		t.Fatalf("state after errors: %+v", got)
	}
}

func TestFolderBrowser(t *testing.T) {
	_, ts, c := newTestServer(t)
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "b-alan"), 0o755)
	os.WriteFile(filepath.Join(root, "b-alan", "devices.csv"), []byte("ip,username,password\n"), 0o600)
	os.MkdirAll(filepath.Join(root, "A-dir"), 0o755)
	os.MkdirAll(filepath.Join(root, ".gizli"), 0o755)
	os.WriteFile(filepath.Join(root, "dosya.txt"), []byte("x"), 0o644)
	var got struct {
		Path    string `json:"path"`
		Parent  string `json:"parent"`
		Entries []struct {
			Name        string `json:"name"`
			Path        string `json:"path"`
			IsWorkspace bool   `json:"is_workspace"`
		} `json:"entries"`
		Roots []string `json:"roots"`
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/fs?path="+root, nil, &got); code != 200 {
		t.Fatalf("status %d", code)
	}
	if got.Path != root || got.Parent != filepath.Dir(root) || len(got.Roots) == 0 {
		t.Fatalf("header: %+v", got)
	}
	if len(got.Entries) != 2 || got.Entries[0].Name != "A-dir" || got.Entries[1].Name != "b-alan" || !got.Entries[1].IsWorkspace || got.Entries[0].IsWorkspace {
		t.Fatalf("entries: %+v", got.Entries)
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/fs?path="+filepath.Join(root, "dosya.txt"), nil, nil); code != http.StatusUnprocessableEntity {
		t.Fatalf("file as dir: %d", code)
	}
	var home struct{ Path string }
	if code := doJSON(t, c, "GET", ts.URL+"/api/fs", nil, &home); code != 200 || home.Path == "" {
		t.Fatalf("default path: %d %q", code, home.Path)
	}
}
