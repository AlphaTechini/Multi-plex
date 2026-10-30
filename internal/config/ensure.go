package config

import (
	"fmt"
	"os"
)

// Ensure creates every directory in p with mode 0700. Existing directories
// are left alone. The runtime directory must belong to the current user.
func (p Paths) Ensure() error {
	for _, dir := range []string{p.DataDir, p.ConfigDir, p.RuntimeDir, p.WorktreesDir, p.LogDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("config: create %s: %w", dir, err)
		}
	}
	return checkOwner(p.RuntimeDir)
}
