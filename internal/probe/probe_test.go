package probe

import (
	"context"
	"strings"
	"testing"
	"time"

	"devupdater/internal/testutil"
)

func TestProbeParsesTools(t *testing.T) {
	s := &testutil.FakeSession{Handler: func(cmd string) (string, int) {
		return "HAVE md5sum\nHAVE printf\ngarbage\nHAVE od", 0
	}}
	c, err := Probe(context.Background(), s, "127.0.0.1", 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Has("md5sum") || !c.Has("printf") || !c.Has("od") || c.Has("base64") || c.SFTP {
		t.Fatalf("caps %+v", c)
	}
	if !strings.Contains(s.Cmds[0], "command -v") {
		t.Fatalf("probe cmd %q", s.Cmds[0])
	}
}
