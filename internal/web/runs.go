package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"sync"
	"time"

	"devupdater/internal/model"
	"devupdater/internal/report"
	"devupdater/internal/runner"
	"devupdater/internal/workspace"
)

var (
	errRunActive    = errors.New("Zaten bir çalışma sürüyor.")
	errStalePreview = errors.New("Önizleme güncel değil. Önce dry-run önizlemesini yeniden çalıştırın.")
	errNoRun        = errors.New("Süren bir çalışma yok.")
	reportIDRe      = regexp.MustCompile(`^[0-9]{8}-[0-9]{6}(-[0-9a-f]{4})?$`)
)

func validReportID(id string) bool { return reportIDRe.MatchString(id) }

// webEvent is a runner event plus the saved report ID on run_done.
type webEvent struct {
	runner.Event
	ReportID string `json:"report_id,omitempty"`
}

type RunStatus struct {
	State          string `json:"state"` // idle | running | done
	DryRun         bool   `json:"dry_run"`
	OnlyFailedFrom string `json:"only_failed_from"`
	TotalDevices   int    `json:"total_devices"`
	ReportID       string `json:"report_id"`
	PreviewID      string `json:"preview_id"`
	PreviewFresh   bool   `json:"preview_fresh"`
	Error          string `json:"error"`
}

type runRequest struct {
	DryRun         bool   `json:"dry_run"`
	OnlyFailedFrom string `json:"only_failed_from"`
	PreviewID      string `json:"preview_id"`
}

type runInputs struct {
	devices  []workspace.Device
	files    []model.LocalFile
	settings workspace.Settings
}

func loadInputs(ws workspace.Workspace) (runInputs, error) {
	var in runInputs
	var err error
	if in.settings, err = workspace.LoadSettings(ws.SettingsPath()); err != nil {
		return in, err
	}
	if in.devices, err = workspace.LoadDevices(ws.DevicesPath()); err != nil {
		return in, err
	}
	if len(in.devices) == 0 {
		return in, errors.New("Cihaz listesi boş.")
	}
	es, err := workspace.LoadManifest(ws.ManifestPath())
	if err != nil {
		return in, err
	}
	in.files, err = workspace.ReadFiles(ws.Dir, es)
	return in, err
}

// fingerprint identifies everything a run's outcome depends on. It stays
// server-side (it covers passwords) and is only compared, never returned.
func fingerprint(in runInputs) string {
	h := sha256.New()
	enc := json.NewEncoder(h)
	enc.Encode(in.devices)
	for _, f := range in.files {
		enc.Encode([]string{f.Remote, f.Mode, f.SHA256})
	}
	enc.Encode(in.settings)
	return hex.EncodeToString(h.Sum(nil))
}

type runManager struct {
	mu      sync.Mutex
	gen     int // bumps when events reset, so old SSE streams end
	status  RunStatus
	events  []webEvent
	changed chan struct{} // closed and replaced on every change
	cancel  context.CancelFunc
	done    chan struct{}
	preview struct{ id, fp string }
}

func newRunManager() *runManager {
	return &runManager{status: RunStatus{State: "idle"}, changed: make(chan struct{})}
}

func (m *runManager) notifyLocked() {
	close(m.changed)
	m.changed = make(chan struct{})
}

func (m *runManager) active() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status.State == "running"
}

// reset forgets the last run and preview (workspace switched). No-op while running.
func (m *runManager) reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State == "running" {
		return
	}
	m.gen++
	m.status = RunStatus{State: "idle"}
	m.events = nil
	m.preview.id, m.preview.fp = "", ""
	m.notifyLocked()
}

func (m *runManager) start(ws workspace.Workspace, req runRequest, job runner.Job, fp string) (RunStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State == "running" {
		return m.status, errRunActive
	}
	if !req.DryRun && req.OnlyFailedFrom == "" &&
		(req.PreviewID == "" || req.PreviewID != m.preview.id || fp != m.preview.fp) {
		return m.status, errStalePreview
	}
	if !req.DryRun {
		// A live run changes device state, so the preview it relied on (or,
		// for a retry, the one that predates it) no longer describes reality.
		m.preview.id, m.preview.fp = "", ""
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	m.cancel, m.done = cancel, done
	m.gen++
	m.events = nil
	m.status = RunStatus{State: "running", DryRun: req.DryRun, OnlyFailedFrom: req.OnlyFailedFrom, TotalDevices: len(job.Devices)}
	m.notifyLocked()
	go m.run(ctx, done, ws, req, job, fp)
	return m.status, nil
}

func (m *runManager) run(ctx context.Context, done chan struct{}, ws workspace.Workspace, req runRequest, job runner.Job, fp string) {
	defer close(done)
	res := runner.Run(ctx, job, func(e runner.Event) {
		if e.Type == "run_done" {
			return // re-emitted below with the report ID
		}
		m.mu.Lock()
		m.events = append(m.events, webEvent{Event: e})
		m.notifyLocked()
		m.mu.Unlock()
	})
	reportID, errText := "", ""
	if _, err := report.Save(ws.ReportsDir(), res); err != nil {
		errText = "Rapor kaydedilemedi: " + err.Error()
	} else {
		reportID = res.ID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.DryRun && req.OnlyFailedFrom == "" && reportID != "" && ctx.Err() == nil {
		m.preview.id, m.preview.fp = reportID, fp
	}
	m.cancel()
	m.status.State, m.status.ReportID, m.status.Error = "done", reportID, errText
	m.events = append(m.events, webEvent{Event: runner.Event{Type: "run_done", Run: &res}, ReportID: reportID})
	m.notifyLocked()
}

func (m *runManager) cancelRun() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status.State != "running" {
		return errNoRun
	}
	m.cancel()
	return nil
}

// shutdown cancels an active run and waits (bounded) for its report to be written.
func (m *runManager) shutdown(timeout time.Duration) {
	m.mu.Lock()
	running, done := m.status.State == "running", m.done
	if running {
		m.cancel()
	}
	m.mu.Unlock()
	if running {
		select {
		case <-done:
		case <-time.After(timeout):
		}
	}
}

// since returns events from index `from` of generation gen, the state, and a
// channel closed on the next change. stale is true when a newer run replaced
// the events of gen.
func (m *runManager) since(gen, from int) (evs []webEvent, state string, changed <-chan struct{}, stale bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if gen != m.gen {
		return nil, m.status.State, m.changed, true
	}
	if from < len(m.events) {
		evs = append(evs, m.events[from:]...)
	}
	return evs, m.status.State, m.changed, false
}

func (m *runManager) snapshot() (RunStatus, int, string, string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := m.status
	st.PreviewID = m.preview.id
	return st, m.gen, m.preview.id, m.preview.fp
}
