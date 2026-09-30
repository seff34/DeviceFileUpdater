package syncer

import (
	"context"
	"fmt"
	"path"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/shell"
	"devupdater/internal/transport"
	"devupdater/internal/upload"
)

const (
	NoteNotCompared = "not compared: device has no hash tool"
	NoteModeUnknown = "existing mode unreadable, used 0644"
)

type Device struct {
	S          transport.Session
	Hasher     Hasher // nil: device cannot hash or read back files
	Chain      *upload.Chain
	CmdTimeout time.Duration
}

type Options struct{ DryRun, Backup bool }

// minUploadRate (bytes/s) sizes the whole-upload deadline: cmdTimeout plus the
// file size at this rate. 32 KiB/s is slow enough for the shell uploaders on
// weak devices, yet a stalled sftp/scp/ftp transfer is abandoned instead of
// hanging the device worker forever.
const minUploadRate = 32 << 10

// uploadTimeout is the deadline for uploading size bytes.
func uploadTimeout(cmdTimeout time.Duration, size int) time.Duration {
	if cmdTimeout <= 0 {
		cmdTimeout = defaultCmdTimeout
	}
	return cmdTimeout + time.Duration(size)*time.Second/minUploadRate
}

func (d *Device) run(ctx context.Context, cmd string) error {
	_, err := exec(ctx, d.S, d.CmdTimeout, cmd)
	return err
}

func (d *Device) SyncFile(ctx context.Context, f model.LocalFile, opt Options) (res model.FileResult) {
	start := time.Now()
	res.Remote = f.Remote
	defer func() { res.DurationMS = time.Since(start).Milliseconds() }()
	fail := func(step string, err error) model.FileResult {
		res.Status, res.Error = model.Failed, fmt.Sprintf("%s: %v", step, err)
		return res
	}

	kind, err := probeType(ctx, d.S, d.CmdTimeout, f.Remote)
	if err != nil {
		return fail("check exists", err)
	}
	switch kind {
	case kindSymlink:
		return fail("check target", fmt.Errorf("target is a symlink; list the real path in the manifest"))
	case kindDir, kindOther:
		return fail("check target", fmt.Errorf("target exists and is not a regular file"))
	}
	existed := kind == kindFile
	if existed {
		if d.Hasher == nil {
			res.Note = NoteNotCompared
		} else {
			rh, err := d.Hasher.Remote(ctx, f.Remote)
			if err != nil {
				return fail("remote hash", err)
			}
			if rh == d.Hasher.Local(f.Data) {
				res.Status = model.Unchanged
				return res
			}
		}
	}
	if opt.DryRun {
		res.Status = model.WouldUpdate
		if !existed {
			res.Status = model.WouldCreate
		}
		return res
	}

	mode := f.Mode
	if mode == "" {
		mode = "0644"
		if existed {
			if m, ok := readMode(ctx, d.S, d.CmdTimeout, f.Remote); ok {
				mode = m
			} else {
				if res.Note != "" {
					res.Note += "; "
				}
				res.Note += NoteModeUnknown
			}
		}
	}

	dst := shell.Quote(f.Remote)
	tmpPath := f.Remote + ".devupd.tmp"
	tmp := shell.Quote(tmpPath)
	cleanup := func() {
		ct := d.CmdTimeout
		if ct <= 0 {
			ct = defaultCmdTimeout
		}
		c, cancel := context.WithTimeout(context.WithoutCancel(ctx), ct)
		defer cancel()
		d.S.Exec(c, "rm -f "+tmp) // best effort; the original is untouched
	}

	if err := d.run(ctx, "mkdir -p "+shell.Quote(path.Dir(f.Remote))); err != nil {
		return fail("mkdir", err)
	}
	// Best effort: a stale temp of equal size must not fool size-checking uploaders.
	_ = d.run(ctx, "rm -f "+tmp)
	if d.Chain == nil {
		return fail("upload", fmt.Errorf("no uploader configured"))
	}
	limit := uploadTimeout(d.CmdTimeout, len(f.Data))
	uctx, cancel := context.WithTimeout(ctx, limit)
	method, err := d.Chain.Upload(uctx, f.Data, tmpPath)
	timedOut := uctx.Err() == context.DeadlineExceeded && ctx.Err() == nil
	cancel()
	res.Method = method
	if err != nil {
		cleanup()
		if timedOut {
			err = fmt.Errorf("timed out after %v: %w", limit, err)
		}
		return fail("upload", err)
	}
	if d.Hasher != nil {
		th, err := d.Hasher.Remote(ctx, tmpPath)
		if err != nil {
			cleanup()
			return fail("verify", err)
		}
		if th != d.Hasher.Local(f.Data) {
			cleanup()
			return fail("verify", fmt.Errorf("uploaded content hash mismatch"))
		}
	}
	if opt.Backup && existed {
		if err := d.run(ctx, "cp -p "+dst+" "+dst+".bak"); err != nil {
			cleanup()
			return fail("backup", err)
		}
	}
	if err := d.run(ctx, "chmod "+mode+" "+tmp); err != nil {
		cleanup()
		return fail("chmod", err)
	}
	if err := d.run(ctx, "mv -f "+tmp+" "+dst); err != nil {
		cleanup()
		return fail("rename", err)
	}
	res.Status = model.Created
	if existed {
		res.Status = model.Updated
	}
	return res
}
