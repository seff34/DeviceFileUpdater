package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"
)

const testToken = "test-token-0123456789"

// newTestServer returns a server with a temp config dir, an httptest server,
// and a client that already holds the session cookie.
func newTestServer(t *testing.T, mut ...func(*Options)) (*Server, *httptest.Server, *http.Client) {
	t.Helper()
	opt := Options{Token: testToken, ConfigDir: t.TempDir()}
	for _, m := range mut {
		m(&opt)
	}
	s, err := New(opt)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Get(ts.URL + "/?token=" + testToken)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("login: status %d", resp.StatusCode)
	}
	return s, ts, c
}

// doJSON sends body as JSON (nil = no body), decodes the response into out
// (nil = ignore) and returns the status code.
func doJSON(t *testing.T, c *http.Client, method, url string, body, out any) int {
	t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode: %v", method, url, err)
		}
	}
	return resp.StatusCode
}
