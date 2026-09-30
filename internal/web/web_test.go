package web

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestAPIRequiresSession(t *testing.T) {
	_, ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/api/workspace")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestWrongTokenRejected(t *testing.T) {
	_, ts, _ := newTestServer(t)
	resp, err := http.Get(ts.URL + "/?token=nope")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestSessionCookieFlags(t *testing.T) {
	_, ts, _ := newTestServer(t)
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(ts.URL + "/?token=" + testToken)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var ck *http.Cookie
	for _, k := range resp.Cookies() {
		if k.Name == sessionCookie {
			ck = k
		}
	}
	if ck == nil || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode || ck.Path != "/" {
		t.Fatalf("bad cookie: %+v", ck)
	}
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Fatalf("redirect to %q", loc)
	}
}

func TestForeignHostRejected(t *testing.T) {
	_, ts, c := newTestServer(t)
	req, _ := http.NewRequest("GET", ts.URL+"/api/workspace", nil)
	req.Host = "evil.example:80"
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestCrossOriginWriteRejected(t *testing.T) {
	_, ts, c := newTestServer(t)
	req, _ := http.NewRequest("POST", ts.URL+"/api/workspace", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://evil.example")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestSPAFallback(t *testing.T) {
	_, ts, c := newTestServer(t)
	for _, p := range []string{"/", "/devices", "/reports/20260930-101010-abcd"} {
		resp, err := c.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", p, resp.StatusCode)
		}
		if !strings.Contains(string(b), "<") {
			t.Fatalf("%s: not HTML: %q", p, b)
		}
	}
}

func TestUnknownAPIIs404JSON(t *testing.T) {
	_, ts, c := newTestServer(t)
	var e struct{ Error string }
	if code := doJSON(t, c, "GET", ts.URL+"/api/nope", nil, &e); code != http.StatusNotFound || e.Error == "" {
		t.Fatalf("got %d %q", code, e.Error)
	}
}

func TestFreshServerHasNoWorkspace(t *testing.T) {
	s, _, _ := newTestServer(t)
	if _, ok := s.workspace(); ok {
		t.Fatal("fresh server must have no workspace")
	}
}

func TestRecentStore(t *testing.T) {
	r := newRecentStore(t.TempDir())
	for _, d := range []string{"/a", "/b", "/a", "/c", "/d", "/e", "/f", "/g", "/h", "/i"} {
		r.add(d)
	}
	got := r.list()
	if len(got) != maxRecent || got[0] != "/i" || got[len(got)-1] != "/a" {
		t.Fatalf("recent: %v", got)
	}
	// persisted
	if again := newRecentStore(r.dir).list(); len(again) != maxRecent || again[0] != "/i" {
		t.Fatalf("not persisted: %v", again)
	}
}
