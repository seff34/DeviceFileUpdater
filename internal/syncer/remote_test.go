package syncer

import "testing"

func TestParseLsMode(t *testing.T) {
	cases := map[string]string{
		"-rwxr-xr-x 1 0 0 12 Jan 1 00:00 /x": "0755",
		"-rw-r--r-- 1 0 0 12 Jan 1 00:00 /x": "0644",
		"-rw------- 1 0 0 12 Jan 1 00:00 /x": "0600",
		"-rwsr-xr-x 1 0 0 12 Jan 1 00:00 /x": "0755",
	}
	for in, want := range cases {
		if got, ok := parseMode(in); !ok || got != want {
			t.Errorf("%q: got %q ok=%v want %s", in, got, ok, want)
		}
	}
	if got, ok := parseMode("755"); !ok || got != "0755" {
		t.Errorf("stat form: %q", got)
	}
	if _, ok := parseMode("garbage"); ok {
		t.Error("garbage must fail")
	}
}
