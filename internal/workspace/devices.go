package workspace

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
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
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = 3
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("devices.csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("devices.csv: empty file")
	}
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
	f, err := os.Create(path)
	if err != nil {
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
		return err
	}
	return f.Close()
}
