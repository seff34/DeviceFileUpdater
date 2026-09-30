//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/runner"
	"devupdater/internal/transport"
	"devupdater/internal/workspace"
)

type devCase struct {
	host, proto, upload, hash string
}

var cases = []devCase{
	{"dev-a", "ssh", "sftp", "sha256sum"},
	{"dev-b", "ssh", "shell-base64", "sha256sum"},
	{"dev-c", "telnet", "ftp", "sha256sum"},
	{"dev-d", "telnet", "shell-printf", "none"},
}

func dev(h string) workspace.Device {
	return workspace.Device{Host: h, Username: "dev", Password: "devpass"}
}

func opts() transport.Options {
	return transport.Options{ConnectTimeout: 5 * time.Second, CommandTimeout: 30 * time.Second}
}

func waitReady(t *testing.T, h string) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		s, err := transport.Dial(context.Background(), dev(h), opts())
		if err == nil {
			s.Close()
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s not ready: %v", h, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func exec(t *testing.T, h, cmd string) string {
	s, err := transport.Dial(context.Background(), dev(h), opts())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	out, _, err := s.Exec(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func lf(remote string, data []byte, mode string) model.LocalFile {
	return model.LocalFile{Remote: remote, Data: data, Mode: mode}
}

func run(h string, files []model.LocalFile, dry bool, post string) model.DeviceResult {
	s := workspace.DefaultSettings()
	s.PostCommand = post
	return runner.Run(context.Background(), runner.Job{
		Devices: []workspace.Device{dev(h)}, Files: files, Settings: s, DryRun: dry,
	}, nil).Devices[0]
}

func statuses(d model.DeviceResult) []model.Status {
	var out []model.Status
	for _, f := range d.Files {
		out = append(out, f.Status)
	}
	return out
}

func TestDeviceMatrix(t *testing.T) {
	bin := make([]byte, 5000)
	rand.Read(bin)
	for _, c := range cases {
		t.Run(c.host, func(t *testing.T) {
			waitReady(t, c.host)
			files := []model.LocalFile{
				lf("/data/app/bin.dat", bin, "0755"),
				lf("/data/conf.txt", []byte("version=1\n"), ""),
			}

			d := run(c.host, files, true, "")
			if d.Error != "" || statuses(d)[0] != model.WouldCreate || statuses(d)[1] != model.WouldCreate {
				t.Fatalf("dry-run: %+v", d)
			}
			if d.Protocol != c.proto || d.HashMethod != c.hash {
				t.Fatalf("protocol/hash: %s/%s", d.Protocol, d.HashMethod)
			}

			d = run(c.host, files, false, "")
			if statuses(d)[0] != model.Created || statuses(d)[1] != model.Created || d.UploadMethod != c.upload {
				t.Fatalf("create: %+v", d)
			}
			if got := strings.TrimSpace(exec(t, c.host, "wc -c < /data/app/bin.dat")); got != "5000" {
				t.Fatalf("bin size %s", got)
			}
			if got := exec(t, c.host, "ls -ln /data/app/bin.dat"); !strings.HasPrefix(got, "-rwxr-xr-x") {
				t.Fatalf("mode: %s", got)
			}

			d = run(c.host, files, false, "")
			want := model.Unchanged
			if c.hash == "none" {
				want = model.Updated // cannot compare: always rewritten
			}
			if statuses(d)[0] != want || statuses(d)[1] != want {
				t.Fatalf("second run: %+v", d)
			}

			files[1] = lf("/data/conf.txt", []byte("version=2\n"), "")
			d = run(c.host, files, false, "cat /data/conf.txt")
			if statuses(d)[1] != model.Updated || d.Post == nil || d.Post.Output != "version=2" {
				t.Fatalf("update: %+v post %+v", d, d.Post)
			}
			if got := exec(t, c.host, "cat /data/conf.txt.bak"); got != "version=1" {
				t.Fatalf("backup: %q", got)
			}
			if got := exec(t, c.host, "ls /data/*.devupd.tmp /data/app/*.devupd.tmp 2>/dev/null | wc -l"); strings.TrimSpace(got) != "0" {
				t.Fatalf("tmp files left: %s", got)
			}

			d = run(c.host, []model.LocalFile{lf("/root/forbidden.txt", []byte("x"), "")}, false, "")
			if statuses(d)[0] != model.Failed {
				t.Fatalf("forbidden path: %+v", d)
			}
		})
	}
}

func TestWrongPassword(t *testing.T) {
	waitReady(t, "dev-a")
	d := runner.Run(context.Background(), runner.Job{
		Devices:  []workspace.Device{{Host: "dev-a", Username: "dev", Password: "wrong"}},
		Files:    []model.LocalFile{lf("/data/x", []byte("x"), "")},
		Settings: workspace.DefaultSettings(),
	}, nil).Devices[0]
	if !strings.Contains(d.Error, "authentication failed") {
		t.Fatalf("%+v", d)
	}
}
