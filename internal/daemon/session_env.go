package daemon

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/safety/envscrub"
)

// sessionConfigDir is the per-session directory the adapter writes its
// config into. It is under the data directory, outside every worktree.
func (d *Daemon) sessionConfigDir(id core.SessionID) string {
	return filepath.Join(d.paths.DataDir, "sessions", string(id))
}

// sessionEnv builds the environment an agent or shell of the session starts
// with (docs/04-safety.md Layer 3): the daemon's environment with credential
// variables removed, plus MPX_SESSION_ID, MPX_SOCKET and MPX_PORT_BASE.
func (d *Daemon) sessionEnv(id core.SessionID, portBase int) []string {
	env := envscrub.ScrubWith(os.Environ(), envscrub.Options{ExtraDeny: d.settings.ExtraScrubEnv}).Env
	env = setEnv(env, "MPX_SESSION_ID", string(id))
	if addr := d.Addr(); addr != "" {
		env = setEnv(env, "MPX_SOCKET", addr)
	}
	if portBase > 0 {
		env = setEnv(env, "MPX_PORT_BASE", strconv.Itoa(portBase))
	}
	return env
}

// setEnv sets name=value in env, replacing an existing entry of that name
// (case-insensitively on Windows).
func setEnv(env []string, name, value string) []string {
	out := env[:0:0]
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if k == name || (runtime.GOOS == "windows" && strings.EqualFold(k, name)) {
			continue
		}
		out = append(out, kv)
	}
	return append(out, name+"="+value)
}
