package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsDefaultsWhenMissing(t *testing.T) {
	s, err := LoadSettings(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil || s != DefaultSettings() {
		t.Fatalf("got %+v err %v", s, err)
	}
}

func TestSettingsPartialFileKeepsDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(p, []byte(`{"parallel": 3}`), 0o644)
	s, err := LoadSettings(p)
	if err != nil || s.Parallel != 3 || !s.Backup || s.CommandTimeoutSec != 30 {
		t.Fatalf("got %+v err %v", s, err)
	}
}

func TestSettingsValidate(t *testing.T) {
	s := DefaultSettings()
	s.PostCommandPolicy = "sometimes"
	if s.Validate() == nil {
		t.Fatal("expected policy error")
	}
	s = DefaultSettings()
	s.Parallel = 0
	if s.Validate() == nil {
		t.Fatal("expected parallel error")
	}
}
