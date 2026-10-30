package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnvHome is the environment variable that moves every directory under one folder.
const EnvHome = "MPX_HOME"

// DefaultPaths returns the platform's Multiplex directories, or every
// directory under $MPX_HOME when that variable is set. It does not create them;
// call Paths.Ensure for that.
func DefaultPaths() (Paths, error) {
	if home := os.Getenv(EnvHome); home != "" {
		abs, err := filepath.Abs(home)
		if err != nil {
			return Paths{}, fmt.Errorf("config: resolve %s: %w", EnvHome, err)
		}
		return pathsUnder(abs), nil
	}

	dataDir, runtimeDir, err := platformDirs()
	if err != nil {
		return Paths{}, err
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return Paths{}, fmt.Errorf("config: user config dir: %w", err)
	}
	return newPaths(dataDir, filepath.Join(configDir, "multiplex"), runtimeDir), nil
}

// pathsUnder lays out every directory beneath a single root (MPX_HOME).
func pathsUnder(root string) Paths {
	return newPaths(filepath.Join(root, "data"), filepath.Join(root, "config"), filepath.Join(root, "run"))
}

// newPaths fills the derived paths from the three base directories.
func newPaths(dataDir, configDir, runtimeDir string) Paths {
	return Paths{
		DataDir:      dataDir,
		ConfigDir:    configDir,
		RuntimeDir:   runtimeDir,
		DBPath:       filepath.Join(dataDir, "multiplex.db"),
		WorktreesDir: filepath.Join(dataDir, "worktrees"),
		LogDir:       filepath.Join(dataDir, "logs"),
	}
}
