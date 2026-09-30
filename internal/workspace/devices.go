package workspace

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Device struct {
	Host     string `json:"host"` // ip, hostname, or host:port
	Username string `json:"username"`
	Password string `json:"password"`
}

// Addr returns host:port, using defaultPort when the CSV gave no port.
func (d Device) Addr(defaultPort int) string {
	if _, _, err := net.SplitHostPort(d.Host); err == nil {
		return d.Host
	}
	return net.JoinHostPort(d.Host, strconv.Itoa(defaultPort))
}

// HostOnly strips an optional port.
func (d Device) HostOnly() string {
	if h, _, err := net.SplitHostPort(d.Host); err == nil {
		return h
	}
	return d.Host
}

var deviceHeader = []string{"ip", "username", "password"}

func ParseDevices(r io.Reader) ([]Device, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("devices.csv: %w", err)
	}
	cr := csv.NewReader(bytes.NewReader(data))
	// Excel in many locales saves CSV with ';'. A header with ';' and no ','
	// can only be that form, so switch separators.
	if header, _, _ := strings.Cut(string(data), "\n"); strings.Contains(header, ";") && !strings.Contains(header, ",") {
		cr.Comma = ';'
	}
	cr.FieldsPerRecord = 3
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("devices.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("devices.csv: empty file")
	}
	// Strip BOM from first header cell if present
	rows[0][0] = strings.TrimPrefix(rows[0][0], "\xef\xbb\xbf")
	for i, h := range deviceHeader {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != h {
			return nil, fmt.Errorf("devices.csv: header must be %s", strings.Join(deviceHeader, ","))
		}
	}
	seen := map[string]bool{}
	var out []Device
	for i, row := range rows[1:] {
		d := Device{Host: strings.TrimSpace(row[0]), Username: strings.TrimSpace(row[1]), Password: row[2]}
		line := i + 2
		if d.Host == "" || d.Username == "" {
			return nil, fmt.Errorf("devices.csv line %d: ip and username are required", line)
		}
		if seen[d.Host] {
			return nil, fmt.Errorf("devices.csv line %d: duplicate ip %s", line, d.Host)
		}
		seen[d.Host] = true
		out = append(out, d)
	}
	return out, nil
}

func LoadDevices(path string) ([]Device, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseDevices(f)
}

func SaveDevices(path string, ds []Device) error {
	// Write to temp file in same directory for atomic replace
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".devices*.csv")
	if err != nil {
		return err
	}
	// Set permissions to 0600 (owner read/write only) before writing
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	w := csv.NewWriter(f)
	w.Write(deviceHeader)
	for _, d := range ds {
		w.Write([]string{d.Host, d.Username, d.Password})
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	// Atomic rename
	return os.Rename(f.Name(), path)
}

// ValidateDevices applies the devices.csv rules to an edited list. Row numbers
// in errors are 1-based positions in ds. An empty list is valid (a draft).
func ValidateDevices(ds []Device) error {
	seen := map[string]bool{}
	for i, d := range ds {
		n := i + 1
		host, user := strings.TrimSpace(d.Host), strings.TrimSpace(d.Username)
		if host == "" || user == "" {
			return fmt.Errorf("device row %d: ip and username are required", n)
		}
		if strings.ContainsAny(host, " \t\r\n") {
			return fmt.Errorf("device row %d: ip must not contain spaces: %q", n, host)
		}
		if seen[host] {
			return fmt.Errorf("device row %d: duplicate ip %s", n, host)
		}
		seen[host] = true
	}
	return nil
}
