//go:build !windows

package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
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
		if d.Host == "leak" {
			return nil, errors.New("auth failed for " + d.Password)
		}
		return testutil.LocalShell{}, nil
	}
	opt.Probe = func(_ context.Context, _ transport.Session, host string, _, _ time.Duration) (probe.Caps, error) {
		if host == "probefail" {
			return probe.Caps{}, errors.New("probe rejected secretpw")
		}
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

// readLines decodes NDJSON lines from body into a channel.
func readLines(body *bufio.Scanner) <-chan map[string]any {
	ch := make(chan map[string]any, 16)
	go func() {
		defer close(ch)
		for body.Scan() {
			var m map[string]any
			if json.Unmarshal(body.Bytes(), &m) == nil {
				ch <- m
			}
		}
	}()
	return ch
}

func TestConnectionTestRedactsPassword(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	openTempWorkspace(t, ts.URL, c)
	doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": []device{
		{Host: "leak", Username: "u", Password: "secretpw"}, {Host: "probefail", Username: "u", Password: "secretpw"},
	}}, nil)
	resp, err := c.Post(ts.URL+"/api/test-connection", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	n := 0
	for sc.Scan() {
		line := sc.Text()
		if strings.Contains(line, "secretpw") {
			t.Fatalf("password leaked: %s", line)
		}
		var m map[string]any
		json.Unmarshal(sc.Bytes(), &m)
		if m["done"] == true {
			continue
		}
		n++
		if e, _ := m["error"].(string); !strings.Contains(e, "***") {
			t.Fatalf("expected redacted error, got %s", line)
		}
	}
	if n != 2 {
		t.Fatalf("want 2 results, got %d", n)
	}
}

func TestConnectionTestEmptyHostsMeansAll(t *testing.T) {
	_, ts, c := newTestServer(t, fakeDevices)
	openTempWorkspace(t, ts.URL, c)
	doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": []device{
		{Host: "a", Username: "u"}, {Host: "b", Username: "u"}, {Host: "down", Username: "u"},
	}}, nil)
	resp, err := c.Post(ts.URL+"/api/test-connection", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	seen := map[string]bool{}
	for m := range readLines(bufio.NewScanner(resp.Body)) {
		if h, ok := m["host"].(string); ok {
			seen[h] = true
		}
	}
	if len(seen) != 3 || !seen["a"] || !seen["b"] || !seen["down"] {
		t.Fatalf("seen %v", seen)
	}
}

func TestConnectionTestStreamsIncrementallyAndCancels(t *testing.T) {
	entered := make(chan struct{})
	var once sync.Once
	handlerDone := make(chan struct{})
	_, ts, c := newTestServer(t, fakeDevices, func(opt *Options) {
		fast := opt.Dial
		opt.Dial = func(ctx context.Context, d workspace.Device, o transport.Options) (transport.Session, error) {
			if d.Host != "slow" {
				return fast(ctx, d, o)
			}
			once.Do(func() { close(entered) })
			<-ctx.Done() // released only by request cancellation
			return nil, ctx.Err()
		}
	})
	openTempWorkspace(t, ts.URL, c)
	doJSON(t, c, "PUT", ts.URL+"/api/devices", map[string]any{"devices": []device{
		{Host: "fast", Username: "u"}, {Host: "slow", Username: "u"},
	}}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", ts.URL+"/api/test-connection", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := readLines(bufio.NewScanner(resp.Body))

	select {
	case m := <-lines:
		if m["host"] != "fast" || m["ok"] != true {
			t.Fatalf("first line: %v", m)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("first result not flushed while second host still blocked")
	}
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("slow host never dialed")
	}
	// The slow host is still blocked: no done line may exist yet.
	select {
	case m, ok := <-lines:
		if ok && m["done"] == true {
			t.Fatal("done arrived before slow host released")
		}
	default:
	}

	cancel()
	go func() {
		for range lines {
		}
		close(handlerDone)
	}()
	select {
	case <-handlerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not return after cancellation")
	}
}
