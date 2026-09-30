package web

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// openTempWorkspace creates and opens a fresh workspace, returning its dir.
func openTempWorkspace(t *testing.T, ts string, c *http.Client) string {
	t.Helper()
	dir := t.TempDir()
	if code := doJSON(t, c, "POST", ts+"/api/workspace", map[string]any{"path": dir, "create": true}, nil); code != 200 {
		t.Fatalf("open workspace: %d", code)
	}
	return dir
}

type device struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func TestDataNeedsWorkspace(t *testing.T) {
	_, ts, c := newTestServer(t)
	for _, p := range []string{"/api/devices", "/api/manifest", "/api/settings"} {
		if code := doJSON(t, c, "GET", ts.URL+p, nil, nil); code != http.StatusConflict {
			t.Errorf("%s: want 409, got %d", p, code)
		}
	}
}

func TestDevicesRoundTrip(t *testing.T) {
	_, ts, c := newTestServer(t)
	dir := openTempWorkspace(t, ts.URL, c)
	var got struct{ Devices []device }
	doJSON(t, c, "GET", ts.URL+"/api/devices", nil, &got)
	if got.Devices == nil || len(got.Devices) != 0 {
		t.Fatalf("empty list must be [] not null: %+v", got)
	}
	in := []device{{Host: " 10.0.0.1 ", Username: "root", Password: "p,w;1"}, {Host: "10.0.0.2:2222", Username: "admin", Password: ""}}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": in}, &got); code != 200 {
		t.Fatalf("put: %d", code)
	}
	if got.Devices[0].Host != "10.0.0.1" || got.Devices[0].Password != "p,w;1" {
		t.Fatalf("saved: %+v", got.Devices)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "devices.csv"))
	if !strings.Contains(string(b), `10.0.0.1,root,"p,w;1"`) {
		t.Fatalf("csv: %q", b)
	}
	var e struct{ Error string }
	dup := []device{{Host: "1.1.1.1", Username: "a"}, {Host: "1.1.1.1", Username: "b"}}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": dup}, &e); code != http.StatusUnprocessableEntity || e.Error == "" {
		t.Fatalf("dup: %d %q", code, e.Error)
	}
}

func TestDevicesImportExport(t *testing.T) {
	_, ts, c := newTestServer(t)
	openTempWorkspace(t, ts.URL, c)
	csv := "\xef\xbb\xbfip;username;password\n10.0.0.5;root;gizli\n"
	resp, err := c.Post(ts.URL+"/api/devices/import", "text/csv", strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !strings.Contains(string(body), `"host":"10.0.0.5"`) {
		t.Fatalf("import: %d %s", resp.StatusCode, body)
	}
	// import does not save
	var got struct{ Devices []device }
	doJSON(t, c, "GET", ts.URL+"/api/devices", nil, &got)
	if len(got.Devices) != 0 {
		t.Fatal("import must not save")
	}
	resp, _ = c.Post(ts.URL+"/api/devices/import", "text/csv", strings.NewReader("a,b\n"))
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("bad csv: %d", resp.StatusCode)
	}
	doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": []device{{Host: "10.0.0.9", Username: "u", Password: "pw"}}}, nil)
	resp, _ = c.Get(ts.URL + "/api/devices/export")
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Disposition"), "devices.csv") || !strings.Contains(string(body), "10.0.0.9,u,pw") {
		t.Fatalf("export: %q %q", resp.Header.Get("Content-Disposition"), body)
	}
}

func upload(t *testing.T, c *http.Client, url string, files map[string]string) (int, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range files {
		fw, _ := mw.CreateFormFile("file", name)
		fw.Write([]byte(content))
	}
	mw.Close()
	resp, err := c.Post(url, mw.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestUploadAndManifest(t *testing.T) {
	_, ts, c := newTestServer(t)
	dir := openTempWorkspace(t, ts.URL, c)
	code, body := upload(t, c, ts.URL+"/api/files", map[string]string{"app.conf": "hello"})
	if code != 200 || !strings.Contains(body, `"local_path":"files/app.conf"`) || !strings.Contains(body, `"replaced":false`) {
		t.Fatalf("upload: %d %s", code, body)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "files", "app.conf")); string(b) != "hello" {
		t.Fatalf("stored: %q", b)
	}
	code, body = upload(t, c, ts.URL+"/api/files", map[string]string{"app.conf": "hello2"})
	if code != 200 || !strings.Contains(body, `"replaced":true`) {
		t.Fatalf("re-upload: %d %s", code, body)
	}
	code, body = upload(t, c, ts.URL+"/api/files", map[string]string{"../../evil.sh": "x"})
	if code != 200 || !strings.Contains(body, `"local_path":"files/evil.sh"`) {
		t.Fatalf("traversal must be flattened: %d %s", code, body)
	}
	if _, err := os.Stat(filepath.Join(dir, "..", "evil.sh")); err == nil {
		t.Fatal("file escaped the workspace")
	}

	type entry struct {
		LocalPath  string `json:"local_path"`
		RemotePath string `json:"remote_path"`
		Mode       string `json:"mode"`
		File       struct {
			Exists bool   `json:"exists"`
			Size   int64  `json:"size"`
			SHA256 string `json:"sha256"`
		} `json:"file"`
	}
	var got struct{ Entries []entry }
	doJSON(t, c, "GET", ts.URL+"/api/manifest", nil, &got)
	if got.Entries == nil || len(got.Entries) != 0 {
		t.Fatalf("fresh manifest: %+v", got)
	}
	rows := []map[string]string{{"local_path": "files/app.conf", "remote_path": "/etc/app.conf", "mode": "0644"}}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/manifest", map[string]any{"entries": rows}, &got); code != 200 {
		t.Fatalf("put manifest: %d", code)
	}
	if !got.Entries[0].File.Exists || got.Entries[0].File.Size != 6 || len(got.Entries[0].File.SHA256) != 64 {
		t.Fatalf("file info: %+v", got.Entries[0].File)
	}
	var e struct{ Error string }
	bad := []map[string]string{{"local_path": "files/yok.bin", "remote_path": "/opt/x", "mode": ""}}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/manifest", map[string]any{"entries": bad}, &e); code != http.StatusUnprocessableEntity || !strings.Contains(e.Error, "files/yok.bin") {
		t.Fatalf("missing file: %d %q", code, e.Error)
	}
	rel := []map[string]string{{"local_path": "files/app.conf", "remote_path": "etc/x", "mode": ""}}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/manifest", map[string]any{"entries": rel}, &e); code != http.StatusUnprocessableEntity {
		t.Fatalf("relative remote: %d", code)
	}
	if code := doJSON(t, c, "PUT", ts.URL+"/api/manifest", map[string]any{"entries": []any{}}, &got); code != 200 {
		t.Fatalf("empty manifest must be savable as a draft: %d", code)
	}
}

func TestSettingsAPI(t *testing.T) {
	_, ts, c := newTestServer(t)
	openTempWorkspace(t, ts.URL, c)
	var s map[string]any
	doJSON(t, c, "GET", ts.URL+"/api/settings", nil, &s)
	if s["parallel"].(float64) != 10 || s["post_command_policy"] != "on_change" {
		t.Fatalf("defaults: %v", s)
	}
	s["parallel"] = 4
	s["post_command"] = "sync"
	if code := doJSON(t, c, "PUT", ts.URL+"/api/settings", s, &s); code != 200 || s["parallel"].(float64) != 4 {
		t.Fatalf("put: %d %v", code, s)
	}
	s["parallel"] = 0
	var e struct{ Error string }
	if code := doJSON(t, c, "PUT", ts.URL+"/api/settings", s, &e); code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid: %d", code)
	}
}
