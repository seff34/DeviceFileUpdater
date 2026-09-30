//go:build !windows

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

// fakeDevices makes host "down" unreachable and every other host a local shell.
func fakeDevices(opt *Options) {
	opt.Dial = func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
		if d.Host == "down" {
			return nil, errors.New("connection refused")
		}
		return testutil.LocalShell{}, nil
	}
	opt.Probe = func(context.Context, transport.Session, string, time.Duration, time.Duration) (probe.Caps, error) {
		return probe.Caps{Tools: map[string]bool{"base64": true, "sha256sum": true}}, nil
	}
}

func TestConnectionTestStreams(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	openTempWorkspace(t, ts.URL, c)
	doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": []device{
		{Host: "up1", Username: "u", Password: "secretpw"}, {Host: "down", Username: "u", Password: "secretpw"}, {Host: "up2", Username: "u"},
	}}, nil)

	resp, err := c.Post(ts.URL+"/api/test-connection", "application/json", strings.NewReader(`{"hosts":["up1","down"]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/x-ndjson") {
		t.Fatalf("content type %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	results := map[string]map[string]any{}
	done := false
	for sc.Scan() {
		var m map[string]any
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			t.Fatalf("bad line %q: %v", sc.Text(), err)
		}
		if m["done"] == true {
			done = true
			continue
		}
		results[m["host"].(string)] = m
		if strings.Contains(sc.Text(), "secretpw") {
			t.Fatal("password leaked into check output")
		}
	}
	if !done || len(results) != 2 {
		t.Fatalf("done=%v results=%v", done, results)
	}
	if results["up1"]["ok"] != true || results["up1"]["hash_method"] != "sha256sum" {
		t.Fatalf("up1: %v", results["up1"])
	}
	if results["down"]["ok"] == true || !strings.HasPrefix(results["down"]["error"].(string), "Cihaza ulaşılamadı") {
		t.Fatalf("down: %v", results["down"])
	}
}

func TestConnectionTestUnknownHosts(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	openTempWorkspace(t, ts.URL, c)
	var e struct{ Error string }
	if code := doJSON(t, c, "POST", ts.URL+"/api/test-connection", map[string]any{"hosts": []string{"nope"}}, &e); code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", code)
	}
}
