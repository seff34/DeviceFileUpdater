package web

import (
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"devupdater/internal/workspace"
)

var errNotFound = errors.New("not found")

func (s *Server) registerWorkspace() {
	s.mux.HandleFunc("GET /api/workspace", s.getWorkspace)
	s.mux.HandleFunc("POST /api/workspace", s.postWorkspace)
	s.mux.HandleFunc("GET /api/fs", s.getFS)
}

// requireWorkspace writes 409 and returns false when no workspace is open.
func (s *Server) requireWorkspace(w http.ResponseWriter) (workspace.Workspace, bool) {
	ws, ok := s.workspace()
	if !ok {
		writeError(w, http.StatusConflict, "Önce bir çalışma alanı seçin.")
	}
	return ws, ok
}

func (s *Server) getWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, _ := s.workspace()
	writeJSON(w, http.StatusOK, map[string]any{"current": ws.Dir, "recent": s.recent.list()})
}

func (s *Server) postWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path   string `json:"path"`
		Create bool   `json:"create"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if s.runActive() {
		writeError(w, http.StatusConflict, "Çalışma sürerken çalışma alanı değiştirilemez.")
		return
	}
	p := strings.TrimSpace(req.Path)
	if p == "" || !filepath.IsAbs(p) {
		writeError(w, http.StatusUnprocessableEntity, "Klasör yolu tam (mutlak) olmalı.")
		return
	}
	p = filepath.Clean(p)
	if err := s.openWorkspace(p, req.Create); err != nil {
		code := http.StatusInternalServerError
		msg := "Çalışma alanı açılamadı: " + err.Error()
		if errors.Is(err, errNotFound) {
			code, msg = http.StatusNotFound, "Klasör bulunamadı: "+p
		}
		writeError(w, code, msg)
		return
	}
	s.getWorkspace(w, r)
}

// openWorkspace opens dir, or with create makes it and fills in missing
// workspace files (existing files are never overwritten).
func (s *Server) openWorkspace(dir string, create bool) error {
	if create {
		ws := workspace.Workspace{Dir: dir}
		for _, d := range []string{dir, ws.FilesDir(), ws.ReportsDir()} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return err
			}
		}
		missing := func(p string) bool { _, err := os.Stat(p); return errors.Is(err, fs.ErrNotExist) }
		if missing(ws.DevicesPath()) {
			if err := workspace.SaveDevices(ws.DevicesPath(), nil); err != nil {
				return err
			}
		}
		if missing(ws.ManifestPath()) {
			if err := workspace.SaveManifest(ws.ManifestPath(), nil); err != nil {
				return err
			}
		}
		if missing(ws.SettingsPath()) {
			if err := workspace.SaveSettings(ws.SettingsPath(), workspace.DefaultSettings()); err != nil {
				return err
			}
		}
	}
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return errNotFound
	}
	s.setWorkspace(dir)
	return nil
}

func isWorkspaceDir(p string) bool {
	for _, f := range []string{"devices.csv", "manifest.csv"} {
		if _, err := os.Stat(filepath.Join(p, f)); err == nil {
			return true
		}
	}
	return false
}

func fsRoots() []string {
	var roots []string
	if runtime.GOOS == "windows" {
		for c := 'A'; c <= 'Z'; c++ {
			d := string(c) + `:\`
			if _, err := os.Stat(d); err == nil {
				roots = append(roots, d)
			}
		}
	} else {
		roots = append(roots, "/")
	}
	if h, err := os.UserHomeDir(); err == nil {
		roots = append(roots, h)
	}
	return roots
}

func (s *Server) getFS(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			h = fsRoots()[0]
		}
		p = h
	}
	if !filepath.IsAbs(p) {
		writeError(w, http.StatusUnprocessableEntity, "Klasör yolu tam (mutlak) olmalı.")
		return
	}
	p = filepath.Clean(p)
	st, err := os.Stat(p)
	if err != nil {
		writeError(w, http.StatusNotFound, "Klasör bulunamadı: "+p)
		return
	}
	if !st.IsDir() {
		writeError(w, http.StatusUnprocessableEntity, "Bu bir klasör değil: "+p)
		return
	}
	ents, err := os.ReadDir(p)
	if err != nil {
		writeError(w, http.StatusForbidden, "Klasör okunamadı: "+err.Error())
		return
	}
	type entry struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		IsWorkspace bool   `json:"is_workspace"`
	}
	out := []entry{}
	for _, e := range ents {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(p, e.Name())
		if st, err := os.Stat(full); err != nil || !st.IsDir() { // follows symlinks to dirs
			continue
		}
		out = append(out, entry{Name: e.Name(), Path: full, IsWorkspace: isWorkspaceDir(full)})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	parent := filepath.Dir(p)
	if parent == p {
		parent = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": p, "parent": parent, "entries": out, "roots": fsRoots()})
}

// runActive reports whether a run is in progress (wired to the run manager in Task 6).
func (s *Server) runActive() bool { return false }
