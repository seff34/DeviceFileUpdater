// Package workspace reads and writes the operator's working folder.
package workspace

import "path/filepath"

type Workspace struct{ Dir string }

func (w Workspace) DevicesPath() string    { return filepath.Join(w.Dir, "devices.csv") }
func (w Workspace) ManifestPath() string   { return filepath.Join(w.Dir, "manifest.csv") }
func (w Workspace) SettingsPath() string   { return filepath.Join(w.Dir, "settings.json") }
func (w Workspace) FilesDir() string       { return filepath.Join(w.Dir, "files") }
func (w Workspace) ReportsDir() string     { return filepath.Join(w.Dir, "reports") }
func (w Workspace) KnownHostsPath() string { return filepath.Join(w.Dir, "known_hosts") }
