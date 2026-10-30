package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformDirs returns the data and runtime directories on macOS.
func platformDirs() (dataDir, runtimeDir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("config: home dir: %w", err)
	}
	dataDir = filepath.Join(home, "Library", "Application Support", "Multiplex")
	return dataDir, tempRuntimeDir(), nil
}

// tempRuntimeDir is the per-user runtime folder under the temp directory.
func tempRuntimeDir() string {
	return filepath.Join(os.TempDir(), fmt.Sprintf("multiplex-%d", os.Getuid()))
}
