//go:build !windows && !darwin

package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformDirs returns the data and runtime directories on Linux and other
// Unix systems, following the XDG base directory specification.
func platformDirs() (dataDir, runtimeDir string, err error) {
	dataDir = os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("config: home dir: %w", err)
		}
		dataDir = filepath.Join(home, ".local", "share")
	}
	dataDir = filepath.Join(dataDir, "multiplex")

	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return dataDir, filepath.Join(xdg, "multiplex"), nil
	}
	return dataDir, filepath.Join(os.TempDir(), fmt.Sprintf("multiplex-%d", os.Getuid())), nil
}
