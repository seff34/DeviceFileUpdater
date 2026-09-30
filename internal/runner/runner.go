// Package runner syncs a manifest to many devices concurrently.
package runner

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"devupdater/internal/model"
	"devupdater/internal/probe"
	"devupdater/internal/syncer"
	"devupdater/internal/transport"
	"devupdater/internal/upload"
	"devupdater/internal/workspace"
)

const (
	defaultConnectTimeout = 10 * time.Second
	defaultCommandTimeout = 30 * time.Second
)

// secs converts a settings value to a duration, falling back when unset or negative.
func secs(n int, def time.Duration) time.Duration {
	if n <= 0 {
		return def
	}
	return time.Duration(n) * time.Second
}

// redact removes the device password from error text destined for
// results/events. Passwords shorter than 4 bytes are not redacted: they would
// corrupt unrelated text (e.g. "1" inside an IP address). Post.Output is
// deliberately left untouched: it is remote stdout of the operator's own command.
func redact(msg, pass string) string {
	if len(pass) < 4 {
		return msg
	}
	return strings.ReplaceAll(msg, pass, "***")
}

// truncate cuts s to at most n bytes without splitting a UTF-8 rune.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "…"
}

type DialFunc func(ctx context.Context, d workspace.Device, opt transport.Options) (transport.Session, error)
type ProbeFunc func(ctx context.Context, s transport.Session, host string, timeout time.Duration) (probe.Caps, error)

type Job struct {
	Devices        []workspace.Device
	Files          []model.LocalFile
	Settings       workspace.Settings
	DryRun         bool
	KnownHostsPath string
	Dial           DialFunc
	Probe          ProbeFunc
}

type Event struct {
	Type  string            `json:"type"`
	Host  string            `json:"host,omitempty"`
	Stage string            `json:"stage,omitempty"`
	Done  int               `json:"done"`
	Total int               `json:"total"`
	File  *model.FileResult `json:"file,omitempty"`
	Error string            `json:"error,omitempty"`
	Run   *model.RunResult  `json:"run,omitempty"`
}

const maxPostOutput = 2000

// newRunID returns a chronologically sortable, path-safe ID: a second-resolution
// timestamp plus 4 random hex chars so same-second runs never collide.
func newRunID(t time.Time) string {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		b[0], b[1] = byte(t.Nanosecond()>>8), byte(t.Nanosecond())
	}
	return t.Format("20060102-150405") + "-" + hex.EncodeToString(b[:])
}

func Run(ctx context.Context, job Job, emit func(Event)) model.RunResult {
	if job.Dial == nil {
		job.Dial = transport.Dial
	}
	if job.Probe == nil {
		job.Probe = probe.Probe
	}
	var emitMu sync.Mutex
	send := func(e Event) {
		if emit == nil {
			return
		}
		emitMu.Lock()
		defer emitMu.Unlock()
		emit(e)
	}

	start := time.Now()
	res := model.RunResult{
		ID: newRunID(start), Started: start,
		DryRun: job.DryRun, Parallel: job.Settings.Parallel,
		Devices: make([]model.DeviceResult, len(job.Devices)),
	}
	for _, f := range job.Files {
		res.Files = append(res.Files, f.Remote)
	}

	sem := make(chan struct{}, max(1, job.Settings.Parallel))
	var wg sync.WaitGroup
	for i, d := range job.Devices {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				if ctx.Err() != nil { // select picks randomly when both are ready
					res.Devices[i] = cancelledDevice(job, d, send)
					return
				}
			case <-ctx.Done():
				res.Devices[i] = cancelledDevice(job, d, send)
				return
			}
			res.Devices[i] = runDevice(ctx, job, d, send)
		}()
	}
	wg.Wait()
	res.Finished = time.Now()
	send(Event{Type: "run_done", Run: &res})
	return res
}

func cancelledDevice(job Job, d workspace.Device, send func(Event)) model.DeviceResult {
	dr := model.DeviceResult{Host: d.Host, Error: "cancelled"}
	failAll(&dr, job.Files, 0, "cancelled")
	send(Event{Type: "device_state", Host: d.Host, Stage: "failed", Total: len(job.Files), Error: dr.Error})
	return dr
}

func failAll(dr *model.DeviceResult, files []model.LocalFile, from int, msg string) {
	for _, f := range files[from:] {
		dr.Files = append(dr.Files, model.FileResult{Remote: f.Remote, Status: model.Failed, Error: msg})
	}
}

func runDevice(ctx context.Context, job Job, d workspace.Device, send func(Event)) (dr model.DeviceResult) {
	start := time.Now()
	dr.Host = d.Host
	total := len(job.Files)
	state := func(stage string, done int) {
		send(Event{Type: "device_state", Host: d.Host, Stage: stage, Done: done, Total: total, Error: dr.Error})
	}
	defer func() {
		if r := recover(); r != nil {
			dr.Error = redact(fmt.Sprintf("internal error: %v", r), d.Password)
			dr.Files = nil
			failAll(&dr, job.Files, 0, dr.Error)
		}
		dr.DurationMS = time.Since(start).Milliseconds()
		if dr.Failed() {
			state("failed", len(dr.Files))
		} else {
			state("done", len(dr.Files))
		}
	}()

	st := job.Settings
	cmdTimeout := secs(st.CommandTimeoutSec, defaultCommandTimeout)
	opt := transport.Options{
		ConnectTimeout: secs(st.ConnectTimeoutSec, defaultConnectTimeout),
		CommandTimeout: cmdTimeout,
		StrictHostKey:  st.StrictHostKey,
		KnownHostsPath: job.KnownHostsPath,
	}

	state("connecting", 0)
	s, err := job.Dial(ctx, d, opt)
	if err == nil && s == nil {
		err = errors.New("no session")
	}
	if err != nil {
		dr.Error = redact("connect: "+err.Error(), d.Password)
		msg := "device unreachable"
		switch {
		case ctx.Err() != nil:
			msg = "cancelled"
		case errors.Is(err, transport.ErrHostKey):
			msg = "host key mismatch"
		}
		failAll(&dr, job.Files, 0, msg)
		return
	}
	defer s.Close()
	dr.Protocol = s.Protocol()

	state("probing", 0)
	caps, err := job.Probe(ctx, s, d.HostOnly(), opt.ConnectTimeout)
	if err != nil {
		dr.Error = redact("probe: "+err.Error(), d.Password)
		msg := "probe failed"
		if ctx.Err() != nil {
			msg = "cancelled"
		}
		failAll(&dr, job.Files, 0, msg)
		return
	}
	dr.Tools = caps.List()
	chain := upload.NewChain(upload.Select(s, caps,
		upload.FTPCreds{Host: d.HostOnly(), User: d.Username, Pass: d.Password}, cmdTimeout))
	h := syncer.SelectHasher(s, caps, cmdTimeout)
	dr.HashMethod = "none"
	if h != nil {
		dr.HashMethod = h.Name()
	}
	dev := &syncer.Device{S: s, Hasher: h, Chain: chain, CmdTimeout: cmdTimeout}

	changed := false
	for i, f := range job.Files {
		if ctx.Err() != nil {
			failAll(&dr, job.Files, i, "cancelled")
			break
		}
		state("syncing", i)
		fr := dev.SyncFile(ctx, f, syncer.Options{DryRun: job.DryRun, Backup: st.Backup})
		fr.Error = redact(fr.Error, d.Password)
		dr.Files = append(dr.Files, fr)
		changed = changed || fr.Status == model.Created || fr.Status == model.Updated
		send(Event{Type: "file_result", Host: d.Host, Done: i + 1, Total: total, File: &fr})
	}
	dr.UploadMethod = chain.Primary()

	runPost := !job.DryRun && st.PostCommand != "" && ctx.Err() == nil &&
		(st.PostCommandPolicy == "always" || (st.PostCommandPolicy == "on_change" && changed))
	if runPost {
		state("post_command", total)
		c, cancel := context.WithTimeout(ctx, cmdTimeout)
		out, code, err := s.Exec(c, st.PostCommand)
		cancel()
		out = truncate(out, maxPostOutput)
		dr.Post = &model.PostResult{Command: redact(st.PostCommand, d.Password), ExitCode: code, Output: out}
		if err != nil {
			dr.Post.Error = redact(err.Error(), d.Password)
		}
	}
	return
}
