package web

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"devupdater/internal/report"
)

func (s *Server) registerReports() {
	s.mux.HandleFunc("GET /api/reports", s.listReports)
	s.mux.HandleFunc("GET /api/reports/{id}", s.getReport)
	s.mux.HandleFunc("GET /api/reports/{id}/html", s.getReportHTML)
}

type reportSummary struct {
	ID            string         `json:"id"`
	Started       time.Time      `json:"started"`
	Finished      time.Time      `json:"finished"`
	DryRun        bool           `json:"dry_run"`
	Devices       int            `json:"devices"`
	DevicesFailed int            `json:"devices_failed"`
	ByStatus      map[string]int `json:"by_status"`
}

func (s *Server) listReports(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	out := []reportSummary{}
	ents, err := os.ReadDir(ws.ReportsDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, e := range ents {
		id, isJSON := strings.CutSuffix(e.Name(), ".json")
		if !isJSON || !validReportID(id) {
			continue
		}
		res, err := report.Load(filepath.Join(ws.ReportsDir(), e.Name()))
		if err != nil {
			continue // unreadable report: skip, never fail the list
		}
		c := report.Summarize(res)
		by := map[string]int{}
		for st, n := range c.ByStatus {
			by[string(st)] = n
		}
		out = append(out, reportSummary{ID: res.ID, Started: res.Started, Finished: res.Finished, DryRun: res.DryRun,
			Devices: c.Devices, DevicesFailed: c.DevicesFailed, ByStatus: by})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"reports": out})
}

// reportPath validates the {id} path value and returns the file path with ext.
func (s *Server) reportPath(w http.ResponseWriter, r *http.Request, ext string) (string, bool) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return "", false
	}
	id := r.PathValue("id")
	if !validReportID(id) {
		writeError(w, http.StatusBadRequest, "Geçersiz rapor kimliği.")
		return "", false
	}
	p := filepath.Join(ws.ReportsDir(), id+ext)
	if _, err := os.Stat(p); err != nil {
		writeError(w, http.StatusNotFound, "Rapor bulunamadı: "+id)
		return "", false
	}
	return p, true
}

func (s *Server) getReport(w http.ResponseWriter, r *http.Request) {
	p, ok := s.reportPath(w, r, ".json")
	if !ok {
		return
	}
	res, err := report.Load(p)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "Rapor okunamadı: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getReportHTML(w http.ResponseWriter, r *http.Request) {
	p, ok := s.reportPath(w, r, ".html")
	if !ok {
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+r.PathValue("id")+`.html"`)
	}
	w.Write(b)
}
