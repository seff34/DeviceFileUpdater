package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

const maxRecent = 8

// recentStore remembers recently opened workspaces in dir/recent.json.
// Failures to persist are ignored: the list is a convenience.
type recentStore struct {
	mu  sync.Mutex
	dir string
}

func newRecentStore(dir string) *recentStore { return &recentStore{dir: dir} }

func (r *recentStore) file() string { return filepath.Join(r.dir, "recent.json") }

func (r *recentStore) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.load()
}

func (r *recentStore) load() []string {
	var v struct {
		Recent []string `json:"recent"`
	}
	b, err := os.ReadFile(r.file())
	if err != nil || json.Unmarshal(b, &v) != nil {
		return []string{}
	}
	if v.Recent == nil {
		return []string{}
	}
	return v.Recent
}

func (r *recentStore) add(dir string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []string{dir}
	for _, d := range r.load() {
		if d != dir && len(out) < maxRecent {
			out = append(out, d)
		}
	}
	b, _ := json.MarshalIndent(map[string][]string{"recent": out}, "", "  ")
	if os.MkdirAll(r.dir, 0o755) == nil {
		os.WriteFile(r.file(), b, 0o644)
	}
}
