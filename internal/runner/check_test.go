//go:build !windows

package runner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

func checkJob() Job {
	s := workspace.DefaultSettings()
	s.Parallel = 2
	return Job{
		Settings: s,
		Dial: func(_ context.Context, d workspace.Device, _ transport.Options) (transport.Session, error) {
			switch d.Host {
			case "down":
				return nil, errors.New("connection refused")
			case "badpass":
				return nil, fmt.Errorf("ssh: %w", transport.ErrAuth)
			case "hostkey":
				return nil, fmt.Errorf("ssh: %w", transport.ErrHostKey)
			}
			return testutil.LocalShell{}, nil
		},
		Probe: func(context.Context, transport.Session, string, time.Duration, time.Duration) (probe.Caps, error) {
			return probe.Caps{Tools: map[string]bool{"sha256sum": true, "base64": true}}, nil
		},
	}
}

func TestCheckReportsMethods(t *testing.T) {
	r := Check(context.Background(), checkJob(), workspace.Device{Host: "up", Username: "u", Password: "secretpw"})
	if !r.OK || r.Protocol != "local" || r.HashMethod != "sha256sum" {
		t.Fatalf("unexpected: %+v", r)
	}
	if len(r.UploadMethods) != 2 || r.UploadMethods[0] != "shell-base64" || r.UploadMethods[1] != "shell-printf" {
		t.Fatalf("upload methods: %v", r.UploadMethods)
	}
	if len(r.Tools) != 2 || r.Tools[0] != "base64" || r.Tools[1] != "sha256sum" {
		t.Fatalf("tools must be sorted: %v", r.Tools)
	}
}

func TestCheckFailuresAreClassifiedAndRedacted(t *testing.T) {
	job := checkJob()
	for host, want := range map[string]string{
		"down":    "Cihaza ulaşılamadı",
		"badpass": "Kullanıcı adı veya şifre hatalı",
		"hostkey": "Host key uyuşmuyor",
	} {
		r := Check(context.Background(), job, workspace.Device{Host: host, Username: "u", Password: "secretpw"})
		if r.OK || r.Error == "" {
			t.Fatalf("%s: expected failure, got %+v", host, r)
		}
		if got := r.Error; len(got) < len(want) || got[:len(want)] != want {
			t.Errorf("%s: error %q must start with %q", host, got, want)
		}
	}
}

func TestCheckAllEmitsEveryDevice(t *testing.T) {
	job := checkJob()
	job.Devices = []workspace.Device{{Host: "up"}, {Host: "down"}, {Host: "badpass"}}
	var mu sync.Mutex
	got := map[string]bool{}
	CheckAll(context.Background(), job, func(r CheckResult) {
		mu.Lock()
		got[r.Host] = r.OK
		mu.Unlock()
	})
	if len(got) != 3 || !got["up"] || got["down"] || got["badpass"] {
		t.Fatalf("results: %v", got)
	}
}
