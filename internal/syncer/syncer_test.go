//go:build !windows

package syncer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/testutil"
	"devupdater/internal/upload"
)

func newDev(withHash bool) *Device {
	s := testutil.LocalShell{}
	var h Hasher
	if withHash {
		h = SelectHasher(s, probe.Caps{Tools: map[string]bool{"base64": true}}, 5*time.Second)
	}
	return &Device{
		S: s, Hasher: h, CmdTimeout: 5 * time.Second,
		Chain: upload.NewChain([]upload.Uploader{upload.NewShellPrintf(s, 5*time.Second)}),
	}
}

func lf(remote, content, mode string) model.LocalFile {
	sum := sha256.Sum256([]byte(content))
	return model.LocalFile{Remote: remote, Mode: mode, Data: []byte(content), SHA256: hex.EncodeToString(sum[:])}
}

func modeOf(t *testing.T, p string) os.FileMode {
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	return st.Mode().Perm()
}

func TestCreateUnchangedUpdate(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "new", "dir", "app.conf")
	ctx := context.Background()

	r := d.SyncFile(ctx, lf(dst, "v1", ""), Options{Backup: true})
	if r.Status != model.Created || r.Method != "shell-printf" {
		t.Fatalf("create: %+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "v1" || modeOf(t, dst) != 0o644 {
		t.Fatalf("create content/mode wrong: %q %v", got, modeOf(t, dst))
	}

	r = d.SyncFile(ctx, lf(dst, "v1", ""), Options{Backup: true})
	if r.Status != model.Unchanged || r.Method != "" {
		t.Fatalf("unchanged: %+v", r)
	}

	os.Chmod(dst, 0o755)
	r = d.SyncFile(ctx, lf(dst, "v2", ""), Options{Backup: true})
	if r.Status != model.Updated {
		t.Fatalf("update: %+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "v2" || modeOf(t, dst) != 0o755 {
		t.Fatalf("update content/mode wrong: %q %v", got, modeOf(t, dst))
	}
	if bak, _ := os.ReadFile(dst + ".bak"); string(bak) != "v1" {
		t.Fatalf("backup: %q", bak)
	}
	if _, err := os.Stat(dst + ".devupd.tmp"); err == nil {
		t.Fatal("tmp left behind")
	}
}

func TestExplicitModeAndNoBackup(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "x")
	os.WriteFile(dst, []byte("old"), 0o644)
	r := d.SyncFile(context.Background(), lf(dst, "new", "0600"), Options{Backup: false})
	if r.Status != model.Updated || modeOf(t, dst) != 0o600 {
		t.Fatalf("%+v mode %v", r, modeOf(t, dst))
	}
	if _, err := os.Stat(dst + ".bak"); err == nil {
		t.Fatal("backup must not exist")
	}
}

func TestDryRun(t *testing.T) {
	d := newDev(true)
	dir := t.TempDir()
	existing := filepath.Join(dir, "e")
	os.WriteFile(existing, []byte("old"), 0o644)
	r1 := d.SyncFile(context.Background(), lf(filepath.Join(dir, "n"), "x", ""), Options{DryRun: true})
	r2 := d.SyncFile(context.Background(), lf(existing, "new", ""), Options{DryRun: true})
	if r1.Status != model.WouldCreate || r2.Status != model.WouldUpdate {
		t.Fatalf("%+v %+v", r1, r2)
	}
	if _, err := os.Stat(filepath.Join(dir, "n")); err == nil {
		t.Fatal("dry-run created a file")
	}
	if got, _ := os.ReadFile(existing); string(got) != "old" {
		t.Fatal("dry-run modified a file")
	}
}

func TestNoHasherAlwaysWrites(t *testing.T) {
	d := newDev(false)
	dst := filepath.Join(t.TempDir(), "x")
	os.WriteFile(dst, []byte("same"), 0o644)
	r := d.SyncFile(context.Background(), lf(dst, "same", ""), Options{})
	if r.Status != model.Updated || r.Note != NoteNotCompared {
		t.Fatalf("%+v", r)
	}
}

func TestUnwritableTargetFailsCleanly(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores permissions")
	}
	d := newDev(true)
	dir := filepath.Join(t.TempDir(), "ro")
	os.MkdirAll(dir, 0o755)
	dst := filepath.Join(dir, "x")
	os.WriteFile(dst, []byte("orig"), 0o644)
	os.Chmod(dir, 0o555)
	defer os.Chmod(dir, 0o755)
	r := d.SyncFile(context.Background(), lf(dst, "new", ""), Options{Backup: true})
	if r.Status != model.Failed || r.Error == "" {
		t.Fatalf("%+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "orig" {
		t.Fatal("original modified")
	}
}

func TestStaleTmpRemovedBeforeUpload(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "x")
	os.WriteFile(dst, []byte("old"), 0o644)
	os.WriteFile(dst+".devupd.tmp", []byte("junk-stale"), 0o644)
	r := d.SyncFile(context.Background(), lf(dst, "new", ""), Options{})
	if r.Status != model.Updated {
		t.Fatalf("%+v", r)
	}
	if got, _ := os.ReadFile(dst); string(got) != "new" {
		t.Fatalf("content %q", got)
	}
	if _, err := os.Stat(dst + ".devupd.tmp"); err == nil {
		t.Fatal("tmp left behind")
	}
}

func TestCancelledContextFails(t *testing.T) {
	d := newDev(true)
	dst := filepath.Join(t.TempDir(), "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := d.SyncFile(ctx, lf(dst, "v", ""), Options{})
	if r.Status != model.Failed || r.Error == "" {
		t.Fatalf("%+v", r)
	}
	if _, err := os.Stat(dst); err == nil {
		t.Fatal("file written despite cancel")
	}
}

func TestZeroTimeoutStillWorks(t *testing.T) {
	d := newDev(true)
	d.CmdTimeout = 0
	dst := filepath.Join(t.TempDir(), "x")
	if r := d.SyncFile(context.Background(), lf(dst, "v", ""), Options{}); r.Status != model.Created {
		t.Fatalf("%+v", r)
	}
}
