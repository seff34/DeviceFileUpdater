package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"devupdater/internal/report"
	"devupdater/internal/workspace"
)

func (s *Server) registerRuns() {
	s.mux.HandleFunc("POST /api/runs", s.postRun)
	s.mux.HandleFunc("GET /api/runs/current", s.getRun)
	s.mux.HandleFunc("GET /api/runs/current/events", s.getRunEvents)
	s.mux.HandleFunc("DELETE /api/runs/current", s.deleteRun)
}

func filterHosts(ds []workspace.Device, hosts []string) []workspace.Device {
	keep := map[string]bool{}
	for _, h := range hosts {
		keep[h] = true
	}
	var out []workspace.Device
	for _, d := range ds {
		if keep[d.Host] {
			out = append(out, d)
		}
	}
	return out
}

func (s *Server) postRun(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	var req runRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.OnlyFailedFrom != "" && !validReportID(req.OnlyFailedFrom) {
		writeError(w, http.StatusUnprocessableEntity, "Geçersiz rapor kimliği.")
		return
	}
	in, err := loadInputs(ws)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	fp := fingerprint(in)
	if req.OnlyFailedFrom != "" {
		prev, err := report.Load(filepath.Join(ws.ReportsDir(), req.OnlyFailedFrom+".json"))
		if err != nil {
			writeError(w, http.StatusNotFound, "Rapor bulunamadı: "+req.OnlyFailedFrom)
			return
		}
		in.devices = filterHosts(in.devices, report.FailedHosts(prev))
		if len(in.devices) == 0 {
			writeError(w, http.StatusUnprocessableEntity, "Bu raporda başarısız cihaz yok.")
			return
		}
	}
	job := s.job(ws)
	job.Devices, job.Files, job.Settings, job.DryRun = in.devices, in.files, in.settings, req.DryRun
	st, err := s.runs.start(ws, req, job, fp)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, st)
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	st, _, pid, pfp := s.runs.snapshot()
	if ws, ok := s.workspace(); ok && pid != "" && st.State != "running" {
		if in, err := loadInputs(ws); err == nil {
			st.PreviewFresh = fingerprint(in) == pfp
		}
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) deleteRun(w http.ResponseWriter, r *http.Request) {
	if err := s.runs.cancelRun(); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// getRunEvents streams the current run's events as SSE, replaying from the start.
func (s *Server) getRunEvents(w http.ResponseWriter, r *http.Request) {
	st, gen, _, _ := s.runs.snapshot()
	if st.State == "idle" {
		writeError(w, http.StatusNotFound, "Çalışma yok.")
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	next := 0
	for {
		evs, state, changed, stale := s.runs.since(gen, next)
		if stale {
			return // a newer run started; the client reconnects
		}
		for _, e := range evs {
			b, err := json.Marshal(e)
			if err != nil {
				return
			}
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return
			}
			next++
		}
		fl.Flush()
		if state == "done" && len(evs) == 0 {
			return // everything including run_done has been sent
		}
		if state == "done" {
			continue // loop once more to confirm nothing is left
		}
		select {
		case <-changed:
		case <-ping.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}
