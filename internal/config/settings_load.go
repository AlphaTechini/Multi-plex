package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// LoadSettings reads settings from path. A missing file returns the defaults.
// Keys absent from the file keep their default value, and unknown keys are
// ignored. Malformed JSON is an error.
func LoadSettings(path string) (Settings, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("config: read settings: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return s, nil
	}
	// Decoding into a struct that already holds the defaults merges the file over them.
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return s, nil
}
