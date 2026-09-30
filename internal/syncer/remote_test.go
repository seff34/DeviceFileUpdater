package syncer

import (
	"context"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func TestProbeTypeParsing(t *testing.T) {
	run := func(out string) (string, error) {
		s := &testutil.FakeSession{Handler: func(string) (string, int) { return out, 0 }}
		return probeType(context.Background(), s, time.Second, "/x")
	}
	if k, err := run("banner\nF\n"); err != nil || k != kindFile {
		t.Fatalf("noise: %q %v", k, err)
	}
	if _, err := run("hello"); err == nil {
		t.Fatal("garbage must error")
	}
	if _, err := run(""); err == nil {
		t.Fatal("empty must error")
	}
}

func TestParseLsMode(t *testing.T) {
	cases := map[string]string{
		"-rwxr-xr-x 1 0 0 12 Jan 1 00:00 /x": "0755",
		"-rw-r--r-- 1 0 0 12 Jan 1 00:00 /x": "0644",
		"-rw------- 1 0 0 12 Jan 1 00:00 /x": "0600",
		"-rwsr-xr-x 1 0 0 12 Jan 1 00:00 /x": "4755",
		"drwxrwxrwt 1 0 0 12 Jan 1 00:00 /x": "1777",
		"-rwSr--r-- 1 0 0 12 Jan 1 00:00 /x": "4644",
		"-rwxr-sr-x 1 0 0 12 Jan 1 00:00 /x": "2755",
	}
	for in, want := range cases {
		if got, ok := parseMode(in); !ok || got != want {
			t.Errorf("%q: got %q ok=%v want %s", in, got, ok, want)
		}
	}
	if got, ok := parseMode("755"); !ok || got != "0755" {
		t.Errorf("stat form: %q", got)
	}
	for in, want := range map[string]string{"7": "0007", "44": "0044", "4755": "4755"} {
		if got, ok := parseMode(in); !ok || got != want {
			t.Errorf("%q: got %q ok=%v", in, got, ok)
		}
	}
	if _, ok := parseMode("garbage"); ok {
		t.Error("garbage must fail")
	}
}
