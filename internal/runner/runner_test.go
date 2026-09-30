//go:build !windows

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

func localProbe(context.Context, transport.Session, string, time.Duration) (probe.Caps, error) {
	return probe.Caps{Tools: map[string]bool{"base64": true}}, nil
}

func file(remote, content string) model.LocalFile {
	return model.LocalFile{Remote: remote, Data: []byte(content)}
}

func baseJob(dir string) Job {
	s := workspace.DefaultSettings()
	s.CommandTimeoutSec = 5
	return Job{
		Settings: s,
		Files:    []model.LocalFile{file(filepath.Join(dir, "a.txt"), "A")},
		Probe:    localProbe,
		Dial: func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
			if d.Host == "down" {
				return nil, errors.New("connection refused")
			}
			return testutil.LocalShell{}, nil
		},
	}
}

func TestRunMixedDevices(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Devices = []workspace.Device{{Host: "down"}, {Host: "up"}}
	var mu sync.Mutex
	var types []string
	res := Run(context.Background(), job, func(e Event) {
		mu.Lock()
		types = append(types, e.Type)
		mu.Unlock()
	})
	down, up := res.Devices[0], res.Devices[1]
	if down.Error == "" || len(down.Files) != 1 || down.Files[0].Status != model.Failed {
		t.Fatalf("down: %+v", down)
	}
	if up.Error != "" || up.Files[0].Status != model.Created || up.Protocol != "local" || up.HashMethod != "readback-base64" {
		t.Fatalf("up: %+v", up)
	}
	if types[len(types)-1] != "run_done" || res.ID == "" || res.Finished.IsZero() {
		t.Fatalf("events %v res %+v", types, res)
	}
}

func TestPostCommandOnChange(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "post.ran")
	job := baseJob(dir)
	job.Devices = []workspace.Device{{Host: "up"}}
	job.Settings.PostCommand = "echo hi; touch " + marker

	res := Run(context.Background(), job, nil)
	if p := res.Devices[0].Post; p == nil || p.ExitCode != 0 || p.Output != "hi" {
		t.Fatalf("post: %+v", p)
	}
	os.Remove(marker)
	res = Run(context.Background(), job, nil) // nothing changes now
	if res.Devices[0].Post != nil {
		t.Fatalf("post must not run without changes: %+v", res.Devices[0].Post)
	}
	job.Settings.PostCommandPolicy = "always"
	res = Run(context.Background(), job, nil)
	if res.Devices[0].Post == nil {
		t.Fatal("policy always must run")
	}
	job.DryRun = true
	res = Run(context.Background(), job, nil)
	if res.Devices[0].Post != nil {
		t.Fatal("dry-run must not run post command")
	}
}

func TestParallelLimitAndPanicIsolation(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Settings.Parallel = 2
	job.DryRun = true
	var cur, peak int32
	job.Dial = func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
		n := atomic.AddInt32(&cur, 1)
		defer atomic.AddInt32(&cur, -1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		if d.Host == "boom" {
			panic("driver bug")
		}
		return testutil.LocalShell{}, nil
	}
	job.Devices = []workspace.Device{{Host: "1"}, {Host: "boom"}, {Host: "3"}, {Host: "4"}, {Host: "5"}}
	res := Run(context.Background(), job, nil)
	if peak > 2 {
		t.Fatalf("parallel limit exceeded: %d", peak)
	}
	if res.Devices[1].Error == "" {
		t.Fatal("panic not captured")
	}
	if fs := res.Devices[1].Files; len(fs) != len(job.Files) || fs[0].Status != model.Failed {
		t.Fatalf("panicked device files: %+v", fs)
	}
	if res.Devices[4].Files[0].Status != model.WouldCreate {
		t.Fatalf("other devices affected: %+v", res.Devices[4])
	}
}

func TestCancelledRunReportsCancelled(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Settings.Parallel = 1
	job.DryRun = true
	ctx, cancel := context.WithCancel(context.Background())
	var calls int32
	job.Dial = func(c context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
		atomic.AddInt32(&calls, 1)
		cancel()
		return nil, c.Err()
	}
	job.Devices = []workspace.Device{{Host: "1"}, {Host: "2"}, {Host: "3"}, {Host: "4"}}
	res := Run(ctx, job, nil)
	for i, d := range res.Devices {
		if d.Error == "" || len(d.Files) != 1 || d.Files[0].Status != model.Failed || d.Files[0].Error != "cancelled" {
			t.Fatalf("device %d not reported cancelled: %+v", i, d)
		}
	}
	// Parallel=1: only the first device can be dialed; the ctx re-check after
	// acquiring the slot stops the rest deterministically.
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("dials=%d, want 1", n)
	}
}

// leakySession fails every non-echo command with an error containing the password.
type leakySession struct {
	testutil.LocalShell
	pw string
}

func (l leakySession) Exec(ctx context.Context, cmd string) (string, int, error) {
	if strings.HasPrefix(cmd, "echo") {
		return l.LocalShell.Exec(ctx, cmd)
	}
	return "", -1, errors.New("ftp login failed for " + l.pw)
}

func TestRedaction(t *testing.T) {
	dir := t.TempDir()
	pw := "s3cret"
	job := baseJob(dir)
	job.Dial = func(context.Context, workspace.Device, transport.Options) (transport.Session, error) {
		return leakySession{pw: pw}, nil
	}
	job.Devices = []workspace.Device{{Host: "up", Password: pw}}
	job.Settings.PostCommand = "echo " + pw
	job.Settings.PostCommandPolicy = "always"
	var evFile string
	res := Run(context.Background(), job, func(e Event) {
		if e.Type == "file_result" {
			evFile = e.File.Error
		}
	})
	fr := res.Devices[0].Files[0]
	if fr.Status != model.Failed || !strings.Contains(fr.Error, "***") {
		t.Fatalf("test premise: error should carry marker: %+v", fr)
	}
	if strings.Contains(fr.Error, pw) || strings.Contains(evFile, pw) || !strings.Contains(evFile, "***") {
		t.Fatalf("file error leaks: %q / %q", fr.Error, evFile)
	}
	p := res.Devices[0].Post
	if p == nil || strings.Contains(p.Command, pw) || p.Output != pw {
		t.Fatalf("post: %+v", p) // command redacted, output untouched
	}
}

func TestShortPasswordLeavesTextIntact(t *testing.T) {
	if got := redact("dial 192.168.1.1", "1"); got != "dial 192.168.1.1" {
		t.Fatalf("got %q", got)
	}
	if got := redact("pw abcd!", "abcd"); got != "pw ***!" {
		t.Fatalf("got %q", got)
	}
}

func TestZeroTimeoutsDoNotMeanForever(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Settings.CommandTimeoutSec = 0
	job.Settings.ConnectTimeoutSec = -1
	job.Devices = []workspace.Device{{Host: "up"}}
	var got transport.Options
	job.Dial = func(_ context.Context, _ workspace.Device, o transport.Options) (transport.Session, error) {
		got = o
		return testutil.LocalShell{}, nil
	}
	Run(context.Background(), job, nil)
	if got.ConnectTimeout <= 0 || got.CommandTimeout <= 0 {
		t.Fatalf("timeouts not defaulted: %+v", got)
	}
}

func TestPasswordNeverLeaks(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Devices = []workspace.Device{{Host: "down", Username: "u", Password: "s3cret"}}
	job.Dial = func(context.Context, workspace.Device, transport.Options) (transport.Session, error) {
		return nil, errors.New("login u:s3cret failed")
	}
	var evs []Event
	res := Run(context.Background(), job, func(e Event) { evs = append(evs, e) })
	if strings.Contains(res.Devices[0].Error, "s3cret") {
		t.Fatalf("leak in result: %q", res.Devices[0].Error)
	}
	for _, e := range evs {
		if strings.Contains(e.Error, "s3cret") {
			t.Fatalf("leak in event: %+v", e)
		}
	}
}

func TestFileResultEventsAndOrdering(t *testing.T) {
	dir := t.TempDir()
	job := baseJob(dir)
	job.Files = []model.LocalFile{file(filepath.Join(dir, "a"), "1"), file(filepath.Join(dir, "b"), "2")}
	job.Devices = []workspace.Device{{Host: "up"}}
	var n int
	res := Run(context.Background(), job, func(e Event) {
		if e.Type == "file_result" {
			n++
			if e.Total != 2 || e.Done != n || e.File == nil {
				t.Errorf("bad event %+v", e)
			}
		}
	})
	if n != 2 || len(res.Devices[0].Files) != 2 || res.Devices[0].Files[1].Remote != job.Files[1].Remote {
		t.Fatalf("n=%d res=%+v", n, res.Devices[0])
	}
}
