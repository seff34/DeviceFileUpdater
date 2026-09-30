package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"devupdater/internal/web"
)

func stubUI(t *testing.T) (opened *[]string) {
	t.Helper()
	t.Setenv("DEVUPDATER_CONFIG_DIR", t.TempDir()) // never touch the real recent.json
	var urls []string
	oldOpen, oldServe := openBrowser, serveUI
	openBrowser = func(u string) error { urls = append(urls, u); return nil }
	serveUI = func(context.Context, *web.Server) error { return nil }
	t.Cleanup(func() { openBrowser, serveUI = oldOpen, oldServe })
	return &urls
}

func TestNoArgsStartsUI(t *testing.T) {
	opened := stubUI(t)
	var out, errb bytes.Buffer
	if code := Main(nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if len(*opened) != 1 || !strings.HasPrefix((*opened)[0], "http://127.0.0.1:") || !strings.Contains((*opened)[0], "/?token=") {
		t.Fatalf("browser: %v", *opened)
	}
	if !strings.Contains(out.String(), (*opened)[0]) {
		t.Fatalf("URL must be printed for manual opening: %q", out.String())
	}
}

func TestUINoBrowserAndWorkspace(t *testing.T) {
	opened := stubUI(t)
	var out, errb bytes.Buffer
	if code := Main([]string{"ui", "-no-browser", "-workspace", t.TempDir()}, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if len(*opened) != 0 {
		t.Fatal("-no-browser opened a browser")
	}
	if code := Main([]string{"ui", "-no-browser", "-workspace", "/definitely/missing/dir"}, &out, &errb); code != 2 {
		t.Fatalf("bad workspace exit %d", code)
	}
}
