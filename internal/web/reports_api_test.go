package web

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/report"
)

func TestReportsAPI(t *testing.T) {
	_, ts, c := newTestServer(t)
	dir := openTempWorkspace(t, ts.URL, c)
	rd := filepath.Join(dir, "reports")
	older := model.RunResult{ID: "20260930-100000-aaaa", Started: time.Now().Add(-time.Hour), Finished: time.Now().Add(-time.Hour), DryRun: true,
		Files: []string{"/a"}, Devices: []model.DeviceResult{{Host: "10.0.0.1", Files: []model.FileResult{{Remote: "/a", Status: model.WouldCreate}}}}}
	newer := model.RunResult{ID: "20260930-110000-bbbb", Started: time.Now(), Finished: time.Now(),
		Files: []string{"/a"}, Devices: []model.DeviceResult{
			{Host: "10.0.0.1", Files: []model.FileResult{{Remote: "/a", Status: model.Created}}},
			{Host: "10.0.0.2", Error: "connect: refused", Files: []model.FileResult{{Remote: "/a", Status: model.Failed}}},
		}}
	for _, r := range []model.RunResult{older, newer} {
		if _, err := report.Save(rd, r); err != nil {
			t.Fatal(err)
		}
	}
	os.WriteFile(filepath.Join(rd, "garbage.json"), []byte("{"), 0o644)

	var list struct {
		Reports []struct {
			ID            string         `json:"id"`
			DryRun        bool           `json:"dry_run"`
			Devices       int            `json:"devices"`
			DevicesFailed int            `json:"devices_failed"`
			ByStatus      map[string]int `json:"by_status"`
		} `json:"reports"`
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/reports", nil, &list); code != 200 {
		t.Fatalf("list: %d", code)
	}
	if len(list.Reports) != 2 || list.Reports[0].ID != newer.ID || list.Reports[1].ID != older.ID {
		t.Fatalf("order: %+v", list.Reports)
	}
	if r := list.Reports[0]; r.Devices != 2 || r.DevicesFailed != 1 || r.ByStatus["CREATED"] != 1 || r.ByStatus["FAILED"] != 1 {
		t.Fatalf("summary: %+v", r)
	}

	var got model.RunResult
	if code := doJSON(t, c, "GET", ts.URL+"/api/reports/"+newer.ID, nil, &got); code != 200 || got.ID != newer.ID || len(got.Devices) != 2 {
		t.Fatalf("get: %d %+v", code, got)
	}
	resp, _ := c.Get(ts.URL + "/api/reports/" + newer.ID + "/html")
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") || !strings.Contains(string(b), "10.0.0.2") || resp.Header.Get("Content-Disposition") != "" {
		t.Fatalf("html: %q", resp.Header.Get("Content-Type"))
	}
	resp, _ = c.Get(ts.URL + "/api/reports/" + newer.ID + "/html?download=1")
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Disposition"), newer.ID+".html") {
		t.Fatalf("download header: %q", resp.Header.Get("Content-Disposition"))
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/reports/..%2Fsecret", nil, nil); code != http.StatusBadRequest && code != http.StatusNotFound {
		t.Fatalf("traversal id: %d", code)
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/reports/bad-id", nil, nil); code != http.StatusBadRequest {
		t.Fatalf("bad id: %d", code)
	}
	if code := doJSON(t, c, "GET", ts.URL+"/api/reports/20200101-000000", nil, nil); code != http.StatusNotFound {
		t.Fatalf("missing: %d", code)
	}
}
