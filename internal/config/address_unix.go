//go:build !windows

package config

import "path/filepath"

// defaultSocketAddress is the Unix socket path inside the runtime directory.
func defaultSocketAddress(p Paths) string {
	return filepath.Join(p.RuntimeDir, "mpx.sock")
}
