// Package model holds result types shared by syncer, runner, report and web.
package model

import "time"

type Status string

const (
	Unchanged   Status = "UNCHANGED"
	Created     Status = "CREATED"
	Updated     Status = "UPDATED"
	WouldCreate Status = "WOULD_CREATE"
	WouldUpdate Status = "WOULD_UPDATE"
	Failed      Status = "FAILED"
)

// LocalFile is a manifest entry with its content loaded from the host.
type LocalFile struct {
	Local  string `json:"local"`
	Remote string `json:"remote"`
	Mode   string `json:"mode"`
	Data   []byte `json:"-"`
	SHA256 string `json:"sha256"`
}

type FileResult struct {
	Remote     string `json:"remote"`
	Status     Status `json:"status"`
	Method     string `json:"method,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
	Note       string `json:"note,omitempty"`
}

type PostResult struct {
	Command  string `json:"command"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
	Error    string `json:"error,omitempty"`
}

type DeviceResult struct {
	Host         string       `json:"host"`
	Protocol     string       `json:"protocol,omitempty"`
	UploadMethod string       `json:"upload_method,omitempty"`
	HashMethod   string       `json:"hash_method,omitempty"`
	Tools        []string     `json:"tools,omitempty"`
	DurationMS   int64        `json:"duration_ms"`
	Error        string       `json:"error,omitempty"`
	Files        []FileResult `json:"files"`
	Post         *PostResult  `json:"post,omitempty"`
}

// Failed reports whether anything on this device went wrong.
func (d DeviceResult) Failed() bool {
	if d.Error != "" {
		return true
	}
	if d.Post != nil && (d.Post.ExitCode != 0 || d.Post.Error != "") {
		return true
	}
	for _, f := range d.Files {
		if f.Status == Failed {
			return true
		}
	}
	return false
}

type RunResult struct {
	ID       string         `json:"id"`
	Started  time.Time      `json:"started"`
	Finished time.Time      `json:"finished"`
	DryRun   bool           `json:"dry_run"`
	Parallel int            `json:"parallel"`
	Files    []string       `json:"files"`
	Devices  []DeviceResult `json:"devices"`
}
