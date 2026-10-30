package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// platformDirs returns the data and runtime directories on Windows. The
// daemon uses a named pipe, so the runtime directory is only a placeholder
// under the data directory.
func platformDirs() (dataDir, runtimeDir string, err error) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", "", fmt.Errorf("config: home dir: %w", err)
		}
		local = filepath.Join(home, "AppData", "Local")
	}
	dataDir = filepath.Join(local, "Multiplex")
	return dataDir, filepath.Join(dataDir, "run"), nil
}
