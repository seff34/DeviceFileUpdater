// Package report renders run results as JSON, HTML and console text.
package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"devupdater/internal/model"
)

type Counts struct {
	Devices       int
	DevicesFailed int
	ByStatus      map[model.Status]int
}

func Summarize(r model.RunResult) Counts {
	c := Counts{Devices: len(r.Devices), ByStatus: map[model.Status]int{}}
	for _, d := range r.Devices {
		if d.Failed() {
			c.DevicesFailed++
		}
		for _, f := range d.Files {
			c.ByStatus[f.Status]++
		}
	}
	return c
}

func FailedHosts(r model.RunResult) []string {
	var out []string
	for _, d := range r.Devices {
		if d.Failed() {
			out = append(out, d.Host)
		}
	}
	return out
}

// safeID rejects IDs that could escape the report directory.
func safeID(id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") || filepath.Base(id) != id {
		return fmt.Errorf("report: unsafe run id %q", id)
	}
	return nil
}

func Save(dir string, r model.RunResult) (string, error) {
	if err := safeID(r.ID); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	// Render first so a template error leaves no .json without its .html.
	var page bytes.Buffer
	if err := WriteHTML(&page, r); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, r.ID+".json"), b, 0o644); err != nil {
		return "", err
	}
	htmlPath := filepath.Join(dir, r.ID+".html")
	if err := os.WriteFile(htmlPath, page.Bytes(), 0o644); err != nil {
		return "", err
	}
	return htmlPath, nil
}

func Load(path string) (model.RunResult, error) {
	var r model.RunResult
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}
