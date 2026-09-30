//go:build !windows

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

// readSSE collects events from /api/runs/current/events until run_done.
func readSSE(t *testing.T, c *http.Client, base string) []map[string]any {
	t.Helper()
	cl := *c
	cl.Timeout = 20 * time.Second
	resp, err := cl.Get(base + "/api/runs/current/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatalf("sse: %d %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var evs []map[string]any
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &m); err != nil {
			t.Fatal(err)
		}
		evs = append(evs, m)
		if m["type"] == "run_done" {
			return evs
		}
	}
	t.Fatalf("stream ended without run_done: %v", evs)
	return nil
}

type runStatus struct {
	State        string `json:"state"`
	DryRun       bool   `json:"dry_run"`
	TotalDevices int    `json:"total_devices"`
	ReportID     string `json:"report_id"`
	PreviewID    string `json:"preview_id"`
	PreviewFresh bool   `json:"preview_fresh"`
	Error        string `json:"error"`
}

// setupRunWorkspace: devices up1 (+ extra), one file whose remote path is in a temp dir.
func setupRunWorkspace(t *testing.T, ts string, c *http.Client, hosts ...string) (remote string) {
	t.Helper()
	openTempWorkspace(t, ts, c)
	var ds []device
	for _, h := range hosts {
		ds = append(ds, device{Host: h, Username: "u", Password: "secretpw"})
	}
	if code := doJSON(t, c, "PUT", ts+"/api/devices", map[string]any{"devices": ds}, nil); code != 200 {
		t.Fatalf("devices: %d", code)
	}
	if code, body := upload(t, c, ts+"/api/files", map[string]string{"app.conf": "v1"}); code != 200 {
		t.Fatalf("upload: %d %s", code, body)
	}
	remote = filepath.Join(t.TempDir(), "etc", "app.conf")
	rows := []map[string]string{{"local_path": "files/app.conf", "remote_path": remote, "mode": ""}}
	if code := doJSON(t, c, "PUT", ts+"/api/manifest", map[string]any{"entries": rows}, nil); code != 200 {
		t.Fatalf("manifest: %d", code)
	}
	return remote
}

func TestPreviewThenApply(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	remote := setupRunWorkspace(t, ts.URL, c, "up1")
	var e struct{ Error string }

	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": false}, &e); code != http.StatusConflict || !strings.Contains(e.Error, "Önizleme") {
		t.Fatalf("real run without preview: %d %q", code, e.Error)
	}
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": true}, nil); code != http.StatusAccepted {
		t.Fatalf("dry run: %d", code)
	}
	evs := readSSE(t, c, ts.URL)
	last := evs[len(evs)-1]
	previewID, _ := last["report_id"].(string)
	if previewID == "" {
		t.Fatalf("run_done without report_id: %v", last)
	}
	sawWould := false
	for _, ev := range evs {
		if ev["type"] == "file_result" && ev["file"].(map[string]any)["status"] == "WOULD_CREATE" {
			sawWould = true
		}
	}
	if !sawWould {
		t.Fatalf("no WOULD_CREATE in %v", evs)
	}
	if _, err := os.Stat(remote); err == nil {
		t.Fatal("dry run wrote the file")
	}
	var st runStatus
	doJSON(t, c, "GET", ts.URL+"/api/runs/current", nil, &st)
	if st.State != "done" || !st.DryRun || st.PreviewID != previewID || !st.PreviewFresh {
		t.Fatalf("status after preview: %+v", st)
	}

	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": false, "preview_id": previewID}, nil); code != http.StatusAccepted {
		t.Fatalf("apply: %d", code)
	}
	evs = readSSE(t, c, ts.URL)
	if b, err := os.ReadFile(remote); err != nil || string(b) != "v1" {
		t.Fatalf("apply did not write: %q %v", b, err)
	}
	reportID := evs[len(evs)-1]["report_id"].(string)
	doJSON(t, c, "GET", ts.URL+"/api/runs/current", nil, &st)
	if st.ReportID != reportID || st.DryRun {
		t.Fatalf("status after apply: %+v", st)
	}

	// changing a file makes the old preview stale
	upload(t, c, ts.URL+"/api/files", map[string]string{"app.conf": "v2"})
	doJSON(t, c, "GET", ts.URL+"/api/runs/current", nil, &st)
	if st.PreviewFresh {
		t.Fatal("preview must be stale after a file change")
	}
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": false, "preview_id": previewID}, &e); code != http.StatusConflict {
		t.Fatalf("stale preview accepted: %d", code)
	}
}

func TestSingleRunAndCancel(t *testing.T) {
	block := func(opt *Options) {
		fakeDevices(opt)
		inner := opt.Dial
		opt.Dial = func(ctx context.Context, d workspace.Device, o transport.Options) (transport.Session, error) {
			if d.Host == "slow" {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return inner(ctx, d, o)
		}
	}
	_, ts, c := newTestServer(t, block)
	setupRunWorkspace(t, ts.URL, c, "slow")
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": true}, nil); code != http.StatusAccepted {
		t.Fatalf("start: %d", code)
	}
	var e struct{ Error string }
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": true}, &e); code != http.StatusConflict {
		t.Fatalf("second start: %d", code)
	}
	if code := doJSON(t, c, "POST", ts.URL+"/api/workspace", map[string]any{"path": t.TempDir()}, &e); code != http.StatusConflict {
		t.Fatalf("workspace switch during run: %d", code)
	}
	req, _ := http.NewRequest("DELETE", ts.URL+"/api/runs/current", nil)
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("cancel: %d", resp.StatusCode)
	}
	evs := readSSE(t, c, ts.URL)
	run := evs[len(evs)-1]["run"].(map[string]any)
	dev := run["devices"].([]any)[0].(map[string]any)
	f := dev["files"].([]any)[0].(map[string]any)
	if f["status"] != "FAILED" || f["error"] != "cancelled" {
		t.Fatalf("cancelled file: %v", f)
	}
	var st runStatus
	doJSON(t, c, "GET", ts.URL+"/api/runs/current", nil, &st)
	if st.PreviewID != "" {
		t.Fatal("a cancelled dry run must not become the preview")
	}
	req, _ = http.NewRequest("DELETE", ts.URL+"/api/runs/current", nil)
	resp, _ = c.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cancel with no run: %d", resp.StatusCode)
	}
}

func TestRetryOnlyFailed(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	setupRunWorkspace(t, ts.URL, c, "up1", "down")
	doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": true}, nil)
	evs := readSSE(t, c, ts.URL)
	id := evs[len(evs)-1]["report_id"].(string)
	var st runStatus
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": false, "only_failed_from": id}, &st); code != http.StatusAccepted || st.TotalDevices != 1 {
		t.Fatalf("retry: %d %+v", code, st)
	}
	readSSE(t, c, ts.URL)
	var e struct{ Error string }
	for _, bad := range []string{"../x", "2026", "20260930-101010-zzzz"} {
		if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"only_failed_from": bad}, &e); code != http.StatusUnprocessableEntity {
			t.Errorf("%q: want 422, got %d", bad, code)
		}
	}
}

func TestEventsWithoutRunIs404(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	openTempWorkspace(t, ts.URL, c)
	if code := doJSON(t, c, "GET", ts.URL+"/api/runs/current/events", nil, nil); code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", code)
	}
	var st runStatus
	doJSON(t, c, "GET", ts.URL+"/api/runs/current", nil, &st)
	if st.State != "idle" {
		t.Fatalf("state %q", st.State)
	}
}

func TestConnectionTestRejectedDuringRun(t *testing.T) {
	block := func(opt *Options) {
		fakeDevices(opt)
		inner := opt.Dial
		opt.Dial = func(ctx context.Context, d workspace.Device, o transport.Options) (transport.Session, error) {
			if d.Host == "slow" {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			return inner(ctx, d, o)
		}
	}
	_, ts, c := newTestServer(t, block)
	setupRunWorkspace(t, ts.URL, c, "slow")
	if code := doJSON(t, c, "POST", ts.URL+"/api/runs", map[string]any{"dry_run": true}, nil); code != http.StatusAccepted {
		t.Fatalf("start: %d", code)
	}
	var e struct{ Error string }
	if code := doJSON(t, c, "POST", ts.URL+"/api/test-connection", map[string]any{}, &e); code != http.StatusConflict || !strings.Contains(e.Error, "Çalışma sürerken") {
		t.Fatalf("test-connection during run: %d %q", code, e.Error)
	}
	if code := doJSON(t, c, "DELETE", ts.URL+"/api/runs/current", nil, nil); code != http.StatusNoContent {
		t.Fatalf("cancel: %d", code)
	}
	readSSE(t, c, ts.URL) // waits for run_done so the run goroutine finishes
}
