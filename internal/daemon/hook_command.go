package daemon

import (
	"os"
	"path/filepath"
	"runtime"
)

// HookCommand returns the argv prefix agents run for hook events. It prefers
// the small mpx-hook binary next to mpx, whose startup is a fraction of the
// full binary's (the TUI's packages, chroma's lexer registry above all, cost
// ~20 ms of init on every call), and falls back to `mpx hook`.
func HookCommand(mpxPath string) []string {
	if mpxPath == "" {
		return nil
	}
	name := "mpx-hook"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(filepath.Dir(mpxPath), name)
	if fi, err := os.Stat(p); err == nil && fi.Mode().IsRegular() {
		return []string{p}
	}
	return []string{mpxPath, "hook"}
}
