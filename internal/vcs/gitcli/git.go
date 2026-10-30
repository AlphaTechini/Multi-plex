// Package gitcli implements vcs.VCS by running the git executable. Every call
// execs git directly (never through a shell), so it behaves the same under
// PowerShell, cmd and POSIX shells.
package gitcli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/alphatechini/multi-plex/internal/vcs"
)

var _ vcs.VCS = (*Git)(nil)

// Git is the git command-line implementation of vcs.VCS. The zero value is not
// usable; create one with New or NewWithPath. It is safe for concurrent use.
type Git struct {
	path string // executable name or path

	resolveOnce sync.Once
	resolved    string

	verMu  sync.Mutex
	verMaj int // 0 until a probe succeeds
	verMin int
}

// New returns a Git that uses the "git" executable found on PATH.
func New() *Git { return NewWithPath("git") }

// NewWithPath returns a Git that runs the given git executable.
func NewWithPath(gitPath string) *Git {
	if gitPath == "" {
		gitPath = "git"
	}
	return &Git{path: gitPath}
}

// Error is returned when a git command exits unsuccessfully. Its message names
// the git subcommand and the trimmed stderr, never the full argument list
// (arguments such as config values may be sensitive).
type Error struct {
	Subcommand string // for example "worktree" or "rev-parse"
	ExitCode   int    // -1 when git could not be started or was killed
	Stderr     string // trimmed
	Err        error  // underlying exec error
}

// Error implements error.
func (e *Error) Error() string {
	if e.Stderr != "" {
		return "git " + e.Subcommand + ": " + e.Stderr
	}
	return fmt.Sprintf("git %s: %v", e.Subcommand, e.Err)
}

// Unwrap returns the underlying exec error (or the context error).
func (e *Error) Unwrap() error { return e.Err }

// runOpts tunes one git invocation.
type runOpts struct {
	dir      string    // working directory ("" = inherit)
	env      []string  // extra KEY=VALUE entries, applied last
	readOnly bool      // set GIT_OPTIONAL_LOCKS=0 so git takes no optional locks
	stdin    io.Reader // optional
}

// gitEnvStrip lists variables that would redirect git away from cmd.Dir if the
// parent process (a git hook, for example) had them set.
var gitEnvStrip = map[string]bool{
	"GIT_DIR": true, "GIT_WORK_TREE": true, "GIT_INDEX_FILE": true,
	"GIT_OBJECT_DIRECTORY": true, "GIT_COMMON_DIR": true, "GIT_PREFIX": true,
	"GIT_NAMESPACE": true, "GIT_ALTERNATE_OBJECT_DIRECTORIES": true,
	"GIT_EXTERNAL_DIFF": true,
}

func (g *Git) exe() string {
	g.resolveOnce.Do(func() {
		g.resolved = g.path
		if p, err := exec.LookPath(g.path); err == nil {
			g.resolved = p
		}
	})
	return g.resolved
}

// run executes git with args and returns stdout. A non-zero exit becomes an
// *Error carrying the trimmed stderr. Cancelling ctx kills git.
func (g *Git) run(ctx context.Context, o runOpts, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, g.exe(), args...)
	cmd.Dir = o.dir
	cmd.Env = buildEnv(o)
	cmd.Stdin = o.stdin
	cmd.WaitDelay = 2 * time.Second // do not hang on pipes held by grandchildren
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	e := &Error{Subcommand: subcommand(args), ExitCode: -1, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		e.ExitCode = ee.ExitCode()
	}
	if cerr := ctx.Err(); cerr != nil {
		e.Err = cerr
		e.Stderr = ""
	}
	return stdout.Bytes(), e
}

// runString runs git and returns stdout with trailing CR/LF removed.
func (g *Git) runString(ctx context.Context, o runOpts, args ...string) (string, error) {
	out, err := g.run(ctx, o, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\r\n"), nil
}

func buildEnv(o runOpts) []string {
	parent := os.Environ()
	env := make([]string, 0, len(parent)+len(o.env)+4)
	for _, kv := range parent {
		name, _, _ := strings.Cut(kv, "=")
		if gitEnvStrip[strings.ToUpper(name)] {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	if o.readOnly {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	return append(env, o.env...)
}

// subcommand returns the git subcommand in args, skipping global -c/-C options.
func subcommand(args []string) string {
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-c" || a == "-C":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			return a
		}
	}
	return "git"
}

// exitCode returns the git exit code carried by err, or -1.
func exitCode(err error) int {
	var ge *Error
	if errors.As(err, &ge) {
		return ge.ExitCode
	}
	return -1
}

// checkArg rejects values that git would parse as an option.
func checkArg(what, v string) error {
	if v == "" {
		return fmt.Errorf("gitcli: empty %s", what)
	}
	if v[0] == '-' {
		return fmt.Errorf("gitcli: invalid %s %q", what, v)
	}
	return nil
}
