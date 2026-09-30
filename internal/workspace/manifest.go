package workspace

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"devupdater/internal/model"
)

type Entry struct {
	LocalPath  string `json:"local_path"`
	RemotePath string `json:"remote_path"`
	Mode       string `json:"mode"`
}

var manifestHeader = []string{"local_path", "remote_path", "mode"}
var modeRe = regexp.MustCompile(`^[0-7]{3,4}$`)

func LoadManifest(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cr := csv.NewReader(f)
	cr.FieldsPerRecord = 3
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("manifest.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("manifest.csv: empty file")
	}
	// Strip BOM from first header cell if present
	rows[0][0] = strings.TrimPrefix(rows[0][0], "\xef\xbb\xbf")
	for i, h := range manifestHeader {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != h {
			return nil, fmt.Errorf("manifest.csv: header must be %s", strings.Join(manifestHeader, ","))
		}
	}
	var es []Entry
	for _, r := range rows[1:] {
		es = append(es, Entry{
			LocalPath:  strings.TrimSpace(r[0]),
			RemotePath: strings.TrimSpace(r[1]),
			Mode:       strings.TrimSpace(r[2]),
		})
	}
	return es, ValidateEntries(es)
}

func SaveManifest(path string, es []Entry) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write(manifestHeader)
	for _, e := range es {
		w.Write([]string{e.LocalPath, e.RemotePath, e.Mode})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func ValidateEntries(es []Entry) error {
	if len(es) == 0 {
		return fmt.Errorf("manifest: no files")
	}
	seen := map[string]bool{}
	for i, e := range es {
		n := i + 1
		if e.LocalPath == "" {
			return fmt.Errorf("manifest row %d: local_path is required", n)
		}
		if !strings.HasPrefix(e.RemotePath, "/") {
			return fmt.Errorf("manifest row %d: remote_path must be absolute: %q", n, e.RemotePath)
		}
		if strings.IndexFunc(e.RemotePath, func(r rune) bool { return r < 0x20 || r == 0x7f }) >= 0 {
			return fmt.Errorf("manifest row %d: remote_path contains a control character: %q", n, e.RemotePath)
		}
		if e.Mode != "" && !modeRe.MatchString(e.Mode) {
			return fmt.Errorf("manifest row %d: mode must be octal like 0644: %q", n, e.Mode)
		}
		if seen[e.RemotePath] {
			return fmt.Errorf("manifest row %d: duplicate remote_path %s", n, e.RemotePath)
		}
		seen[e.RemotePath] = true
	}
	return nil
}

// ReadFiles loads each entry's content; relative local paths resolve against baseDir.
func ReadFiles(baseDir string, es []Entry) ([]model.LocalFile, error) {
	var out []model.LocalFile
	for _, e := range es {
		p := e.LocalPath
		if !filepath.IsAbs(p) {
			p = filepath.Join(baseDir, p)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
		sum := sha256.Sum256(data)
		out = append(out, model.LocalFile{
			Local: p, Remote: e.RemotePath, Mode: e.Mode,
			Data: data, SHA256: hex.EncodeToString(sum[:]),
		})
	}
	return out, nil
}
