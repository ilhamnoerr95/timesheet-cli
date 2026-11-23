package timesheet

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds settings that outlive a single command: linked timesheet files and which one is active.
type Config struct {
	ActiveSheet string            `json:"active_sheet"`
	Sheets      map[string]string `json:"sheets"` // name -> path of the .xlsx file
}

type JSONConfigStore struct{ Path string }

func (c JSONConfigStore) Load() (Config, error) {
	var cfg Config
	b, err := os.ReadFile(c.Path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	return cfg, json.Unmarshal(b, &cfg)
}

// Save writes via temp file + rename so a crash never leaves a half-written config.
func (c JSONConfigStore) Save(cfg Config) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.Path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.Path), "*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), c.Path)
}
