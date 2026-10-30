// Package xptyimpl implements the pty package's neutral interfaces with
// github.com/charmbracelet/x/xpty: a Unix PTY on Linux and macOS, ConPTY on
// Windows.
package xptyimpl

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/x/xpty"

	"github.com/alphatechini/multi-plex/internal/pty"
)

// Default terminal size used when a Spec leaves Cols or Rows unset.
const (
	defaultCols = 80
	defaultRows = 24
)

// Spawner starts programs in pseudo-terminals using xpty.
type Spawner struct{}

var _ pty.Spawner = (*Spawner)(nil)

// New returns a Spawner.
func New() *Spawner { return &Spawner{} }

// Spawn starts spec.Argv in a new pseudo-terminal. Argv[0] is resolved with
// exec.LookPath (which honours PATHEXT on Windows). If ctx is cancelled the
// process is killed. The child always gets TERM=xterm-256color and
// COLORTERM=truecolor, whatever the environment says.
func (s *Spawner) Spawn(ctx context.Context, spec pty.Spec) (pty.Process, error) {
	if len(spec.Argv) == 0 || spec.Argv[0] == "" {
		return nil, errors.New("xptyimpl: empty argv")
	}
	argv, err := resolveArgv(spec.Argv)
	if err != nil {
		return nil, err
	}
	cols, rows := spec.Cols, spec.Rows
	if cols <= 0 {
		cols = defaultCols
	}
	if rows <= 0 {
		rows = defaultRows
	}

	// xpty ignores its options (v0.1.4), so the size is set explicitly.
	p, err := xpty.NewPty(cols, rows)
	if err != nil {
		return nil, fmt.Errorf("xptyimpl: open pty: %w", err)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = spec.Dir
	cmd.Env = terminalEnv(spec.Env)
	cmd.SysProcAttr = sysProcAttr()

	if err := p.Start(cmd); err != nil {
		_ = p.Close()
		return nil, fmt.Errorf("xptyimpl: start %s: %w", filepath.Base(argv[0]), err)
	}
	// On Unix our copy of the slave must be closed, or reads on the master
	// never end after the child exits.
	closeSlave(p)

	pr := newProcess(p, cmd)
	go pr.reap()
	go func() {
		select {
		case <-ctx.Done():
			_ = pr.Kill()
		case <-pr.done:
		}
	}()
	return pr, nil
}

// resolveArgv resolves Argv[0] on PATH and, on Windows, wraps .cmd and .bat
// shims (npm installs them) in cmd.exe, since ConPTY can only start real
// executables.
func resolveArgv(argv []string) ([]string, error) {
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return nil, fmt.Errorf("xptyimpl: %w", err)
	}
	out := make([]string, len(argv))
	copy(out, argv)
	out[0] = path
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(path)) {
		case ".cmd", ".bat":
			comspec := os.Getenv("ComSpec")
			if comspec == "" {
				comspec = "cmd.exe"
			}
			out = append([]string{comspec, "/c"}, out...)
		}
	}
	return out, nil
}

// terminalEnv returns env (or the current environment when nil) with TERM and
// COLORTERM describing the hosting emulator. They are always set, replacing
// any inherited value: the program talks to Multiplex's emulator, not to the
// terminal the daemon happened to be started from (TERM=linux, tmux, ...).
func terminalEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		switch keyName(k) {
		case "TERM", "COLORTERM":
			continue
		}
		out = append(out, kv)
	}
	return append(out, "TERM=xterm-256color", "COLORTERM=truecolor")
}

// keyName normalises an environment variable name for comparison
// (Windows names are case-insensitive).
func keyName(k string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(k)
	}
	return k
}
