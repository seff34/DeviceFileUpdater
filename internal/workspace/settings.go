package workspace

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type Settings struct {
	Parallel          int    `json:"parallel"`
	Backup            bool   `json:"backup"`
	PostCommand       string `json:"post_command"`
	PostCommandPolicy string `json:"post_command_policy"`
	ConnectTimeoutSec int    `json:"connect_timeout_sec"`
	CommandTimeoutSec int    `json:"command_timeout_sec"`
	StrictHostKey     bool   `json:"strict_host_key"`
}

func DefaultSettings() Settings {
	return Settings{
		Parallel: 10, Backup: true, PostCommandPolicy: "on_change",
		ConnectTimeoutSec: 10, CommandTimeoutSec: 30,
	}
}

// LoadSettings returns defaults for a missing file; fields absent from the file keep defaults.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("settings.json: %w", err)
	}
	return s, s.Validate()
}

func SaveSettings(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(path, b, 0o644)
}

func (s Settings) Validate() error {
	if s.Parallel < 1 || s.Parallel > 200 {
		return fmt.Errorf("settings: parallel must be 1..200")
	}
	switch s.PostCommandPolicy {
	case "on_change", "always", "never":
	default:
		return fmt.Errorf("settings: post_command_policy must be on_change, always or never")
	}
	if s.ConnectTimeoutSec < 1 || s.CommandTimeoutSec < 1 {
		return fmt.Errorf("settings: timeouts must be >= 1 second")
	}
	return nil
}
