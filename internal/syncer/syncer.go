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

	existed, err := exists(ctx, d.S, d.CmdTimeout, f.Remote)
	if err != nil {
		return fail("check exists", err)
	}
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
				res.Note = NoteModeUnknown
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
	method, err := d.Chain.Upload(ctx, f.Data, tmpPath)
	res.Method = method
	if err != nil {
		cleanup()
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
