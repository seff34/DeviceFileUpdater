package syncer

import (
	"context"
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"
)

func TestSelectHasherOrder(t *testing.T) {
	s := &testutil.FakeSession{}
	cases := []struct {
		tools []string
		want  string
	}{
		{[]string{"md5sum", "sha256sum", "base64"}, "sha256sum"},
		{[]string{"md5sum", "od"}, "md5sum"},
		{[]string{"od", "base64"}, "readback-base64"},
		{[]string{"hexdump", "od"}, "readback-od"},
		{[]string{"hexdump"}, "readback-hexdump"},
	}
	for _, c := range cases {
		caps := probe.Caps{Tools: map[string]bool{}}
		for _, t := range c.tools {
			caps.Tools[t] = true
		}
		if h := SelectHasher(s, caps, time.Second); h == nil || h.Name() != c.want {
			t.Errorf("%v: got %v want %s", c.tools, h, c.want)
		}
	}
	if SelectHasher(s, probe.Caps{Tools: map[string]bool{}}, time.Second) != nil {
		t.Error("expected nil hasher")
	}
}

func TestSumHasherParses(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) {
		return "5D41402ABC4B2A76B9719D911017C592  /x", 0
	}}
	h := SelectHasher(s, probe.Caps{Tools: map[string]bool{"md5sum": true}}, time.Second)
	got, err := h.Remote(context.Background(), "/x")
	if err != nil || got != h.Local([]byte("hello")) {
		t.Fatalf("got %q err %v, local %q", got, err, h.Local([]byte("hello")))
	}
}

func TestReadbackOd(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(string) (string, int) {
		return " 68 65 6c\n 6c 6f", 0
	}}
	h := SelectHasher(s, probe.Caps{Tools: map[string]bool{"od": true}}, time.Second)
	got, err := h.Remote(context.Background(), "/x")
	if err != nil || got != h.Local([]byte("hello")) {
		t.Fatalf("got %q err %v", got, err)
	}
}
