package web

import (
	"encoding/json"
	"net/http"

	"devupdater/internal/runner"
	"devupdater/internal/workspace"
)

func (s *Server) registerCheck() {
	s.mux.HandleFunc("POST /api/test-connection", s.postTestConnection)
}

// job builds a runner job for ws with the server's dial/probe seams.
func (s *Server) job(ws workspace.Workspace) runner.Job {
	return runner.Job{Dial: s.opt.Dial, Probe: s.opt.Probe, KnownHostsPath: ws.KnownHostsPath()}
}

func (s *Server) postTestConnection(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	var req struct {
		Hosts []string `json:"hosts"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if s.runActive() {
		writeError(w, http.StatusConflict, "Çalışma sürerken bağlantı testi yapılamaz.")
		return
	}
	devices, err := loadDevicesOrEmpty(ws)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	settings, err := workspace.LoadSettings(ws.SettingsPath())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if len(req.Hosts) > 0 {
		want := map[string]bool{}
		for _, h := range req.Hosts {
			want[h] = true
		}
		var keep []workspace.Device
		for _, d := range devices {
			if want[d.Host] {
				keep = append(keep, d)
			}
		}
		devices = keep
	}
	if len(devices) == 0 {
		writeError(w, http.StatusUnprocessableEntity, "Test edilecek cihaz yok.")
		return
	}
	job := s.job(ws)
	job.Devices, job.Settings = devices, settings

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	enc := json.NewEncoder(w) // Encode appends '\n'
	flush := func() {
		if fl != nil {
			fl.Flush()
		}
	}
	flush()
	runner.CheckAll(r.Context(), job, func(res runner.CheckResult) {
		enc.Encode(res)
		flush()
	})
	enc.Encode(map[string]bool{"done": true})
	flush()
}
