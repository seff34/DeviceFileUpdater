package web

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"devupdater/internal/workspace"
)

const maxUploadBytes = 4 << 30 // 4 GiB per request

func (s *Server) registerData() {
	s.mux.HandleFunc("GET /api/devices", s.getDevices)
	s.mux.HandleFunc("PUT /api/devices", s.putDevices)
	s.mux.HandleFunc("POST /api/devices/import", s.importDevices)
	s.mux.HandleFunc("GET /api/devices/export", s.exportDevices)
	s.mux.HandleFunc("GET /api/manifest", s.getManifest)
	s.mux.HandleFunc("PUT /api/manifest", s.putManifest)
	s.mux.HandleFunc("POST /api/files", s.postFiles)
	s.mux.HandleFunc("GET /api/settings", s.getSettings)
	s.mux.HandleFunc("PUT /api/settings", s.putSettings)
}

// ---- devices ----

func loadDevicesOrEmpty(ws workspace.Workspace) ([]workspace.Device, error) {
	ds, err := workspace.LoadDevices(ws.DevicesPath())
	if errors.Is(err, fs.ErrNotExist) {
		return []workspace.Device{}, nil
	}
	if ds == nil && err == nil {
		ds = []workspace.Device{}
	}
	return ds, err
}

func (s *Server) getDevices(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	ds, err := loadDevicesOrEmpty(ws)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "devices.csv okunamadı: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": ds})
}

func (s *Server) putDevices(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	var req struct {
		Devices []workspace.Device `json:"devices"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	for i := range req.Devices {
		req.Devices[i].Host = strings.TrimSpace(req.Devices[i].Host)
		req.Devices[i].Username = strings.TrimSpace(req.Devices[i].Username)
	}
	if err := workspace.ValidateDevices(req.Devices); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := workspace.SaveDevices(ws.DevicesPath(), req.Devices); err != nil {
		writeError(w, http.StatusInternalServerError, "devices.csv yazılamadı: "+err.Error())
		return
	}
	s.getDevices(w, r)
}

func (s *Server) importDevices(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.requireWorkspace(w); !ok {
		return
	}
	ds, err := workspace.ParseDevices(http.MaxBytesReader(w, r.Body, 5<<20))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if ds == nil {
		ds = []workspace.Device{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"devices": ds})
}

func (s *Server) exportDevices(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	b, err := os.ReadFile(ws.DevicesPath())
	if errors.Is(err, fs.ErrNotExist) {
		b, err = []byte("ip,username,password\n"), nil
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="devices.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(b)
}

// ---- manifest + files ----

type fileInfo struct {
	Exists bool   `json:"exists"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

type hashKey struct {
	path string
	size int64
	mod  time.Time
}

var (
	hashMu    sync.Mutex
	hashCache = map[hashKey]string{}
)

func resolveLocal(ws workspace.Workspace, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(ws.Dir, filepath.FromSlash(p))
}

// fileInfo stats and hashes a manifest file; hashes are cached by path, size and mtime.
func (s *Server) fileInfo(ws workspace.Workspace, localPath string) fileInfo {
	p := resolveLocal(ws, localPath)
	st, err := os.Stat(p)
	if err != nil || st.IsDir() {
		return fileInfo{}
	}
	k := hashKey{p, st.Size(), st.ModTime()}
	hashMu.Lock()
	sum, ok := hashCache[k]
	hashMu.Unlock()
	if !ok {
		f, err := os.Open(p)
		if err != nil {
			return fileInfo{}
		}
		h := sha256.New()
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return fileInfo{}
		}
		sum = hex.EncodeToString(h.Sum(nil))
		hashMu.Lock()
		hashCache[k] = sum
		hashMu.Unlock()
	}
	return fileInfo{Exists: true, Size: st.Size(), SHA256: sum}
}

type manifestRow struct {
	workspace.Entry
	File fileInfo `json:"file"`
}

func (s *Server) getManifest(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	es, err := workspace.LoadManifestDraft(ws.ManifestPath())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "manifest.csv okunamadı: "+err.Error())
		return
	}
	rows := []manifestRow{}
	for _, e := range es {
		rows = append(rows, manifestRow{Entry: e, File: s.fileInfo(ws, e.LocalPath)})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": rows})
}

func (s *Server) putManifest(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	var req struct {
		Entries []workspace.Entry `json:"entries"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	for i := range req.Entries {
		e := &req.Entries[i]
		e.LocalPath, e.RemotePath, e.Mode = strings.TrimSpace(e.LocalPath), strings.TrimSpace(e.RemotePath), strings.TrimSpace(e.Mode)
	}
	if len(req.Entries) > 0 {
		if err := workspace.ValidateEntries(req.Entries); err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		for i, e := range req.Entries {
			if !s.fileInfo(ws, e.LocalPath).Exists {
				writeError(w, http.StatusUnprocessableEntity, fmt.Sprintf("manifest row %d: dosya bulunamadı: %s", i+1, e.LocalPath))
				return
			}
		}
	}
	if err := workspace.SaveManifest(ws.ManifestPath(), req.Entries); err != nil {
		writeError(w, http.StatusInternalServerError, "manifest.csv yazılamadı: "+err.Error())
		return
	}
	s.getManifest(w, r)
}

// safeFileName flattens an uploaded name to a single path element.
func safeFileName(name string) (string, error) {
	base := filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	base = strings.TrimSpace(base)
	if base == "" || base == "." || base == ".." || base == "/" {
		return "", fmt.Errorf("geçersiz dosya adı: %q", name)
	}
	if strings.IndexFunc(base, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
		return "", fmt.Errorf("dosya adında kontrol karakteri var: %q", name)
	}
	return base, nil
}

func (s *Server) postFiles(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "multipart bekleniyordu: "+err.Error())
		return
	}
	if err := os.MkdirAll(ws.FilesDir(), 0o755); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type saved struct {
		LocalPath string `json:"local_path"`
		Size      int64  `json:"size"`
		SHA256    string `json:"sha256"`
		Replaced  bool   `json:"replaced"`
	}
	out := []saved{}
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "yükleme okunamadı: "+err.Error())
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			part.Close()
			continue
		}
		name, err := safeFileName(part.FileName())
		if err != nil {
			part.Close()
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		dst := filepath.Join(ws.FilesDir(), name)
		_, statErr := os.Stat(dst)
		tmp, err := os.CreateTemp(ws.FilesDir(), ".upload-*")
		if err != nil {
			part.Close()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h := sha256.New()
		n, err := io.Copy(io.MultiWriter(tmp, h), part)
		part.Close()
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmp.Name(), dst)
		}
		if err != nil {
			os.Remove(tmp.Name())
			writeError(w, http.StatusInternalServerError, "dosya kaydedilemedi: "+err.Error())
			return
		}
		out = append(out, saved{LocalPath: "files/" + name, Size: n, SHA256: hex.EncodeToString(h.Sum(nil)), Replaced: statErr == nil})
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

// ---- settings ----

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	st, err := workspace.LoadSettings(ws.SettingsPath())
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	ws, ok := s.requireWorkspace(w)
	if !ok {
		return
	}
	st := workspace.DefaultSettings()
	if !readJSON(w, r, &st) {
		return
	}
	if err := st.Validate(); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := workspace.SaveSettings(ws.SettingsPath(), st); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}
