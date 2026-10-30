// Package shell is the agent adapter for a plain shell. Multiplex uses it
// for terminal-mode panes and for testing sessions without an agent CLI. It
// has no hooks and no MCP registration.
package shell

import (
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// Name is the registry name of this adapter.
const Name = "shell"

func init() { agent.Register(Adapter{}) }

// Adapter runs the user's shell.
type Adapter struct{}

var _ agent.Adapter = Adapter{}

// Name returns "shell".
func (Adapter) Name() string { return Name }

// DisplayName returns the name shown in the UI.
func (Adapter) DisplayName() string { return "Shell" }

// Capabilities reports no capabilities: a shell has no hooks, MCP, sandbox,
// side questions or roles.
func (Adapter) Capabilities() agent.Capabilities { return agent.Capabilities{} }

// Detect reports the shell Prepare would start.
func (Adapter) Detect() (bool, string) {
	argv, err := shellArgv(runtime.GOOS, os.Getenv, exec.LookPath)
	if err != nil {
		return false, err.Error()
	}
	return true, argv[0]
}

// Prepare returns the command line for the user's shell, started in the
// worktree with the session's environment plus MPX_SESSION_ID. A nil
// spec.Env inherits the current environment.
func (Adapter) Prepare(spec agent.LaunchSpec) (agent.Launch, error) {
	argv, err := shellArgv(runtime.GOOS, os.Getenv, exec.LookPath)
	if err != nil {
		return agent.Launch{}, err
	}
	return agent.Launch{
		Argv: argv,
		Env:  withEnv(spec.Env, "MPX_SESSION_ID="+string(spec.Session.ID)),
		Dir:  spec.WorktreePath,
	}, nil
}

// shellArgv picks the shell for an operating system. On Windows it is
// PowerShell 7 if installed, else Windows PowerShell, both with -NoLogo;
// cmd.exe is never chosen. Elsewhere it is $SHELL or /bin/sh.
func shellArgv(goos string, getenv func(string) string, lookPath func(string) (string, error)) ([]string, error) {
	if goos == "windows" {
		for _, exe := range []string{"pwsh.exe", "powershell.exe"} {
			if p, err := lookPath(exe); err == nil {
				return []string{p, "-NoLogo"}, nil
			}
		}
		return nil, errors.New("shell: neither pwsh.exe nor powershell.exe found on PATH")
	}
	if sh := getenv("SHELL"); sh != "" {
		return []string{sh}, nil
	}
	return []string{"/bin/sh"}, nil
}

// withEnv returns base (or the current environment when nil) with the given
// KEY=value entries set, replacing any existing entry of the same name.
func withEnv(base []string, extra ...string) []string {
	if base == nil {
		base = os.Environ()
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		if !hasKey(kv, extra) {
			out = append(out, kv)
		}
	}
	return append(out, extra...)
}

// hasKey reports whether kv sets the same variable as any entry of set.
func hasKey(kv string, set []string) bool {
	k, _, _ := strings.Cut(kv, "=")
	for _, s := range set {
		sk, _, _ := strings.Cut(s, "=")
		if envKeyEqual(k, sk) {
			return true
		}
	}
	return false
}

func envKeyEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// ParseHook always fails: a shell has no hooks.
func (Adapter) ParseHook(event string, payload []byte) (agent.HookEvent, error) {
	return agent.HookEvent{}, errors.New("shell: adapter has no hooks")
}

// EncodeHookResponse produces nothing: a shell has no hooks.
func (Adapter) EncodeHookResponse(agent.HookEvent, agent.HookResponse) ([]byte, int) {
	return nil, 0
}

// FailClosed produces nothing and exits 2, though a shell never calls hooks.
func (Adapter) FailClosed(string) ([]byte, int) { return nil, 2 }

// TypeMessage types text into the shell and presses Enter. A multi-line
// message (review comments, which quote code) is typed as comment lines, one
// "# " line each, so a shell never runs the quoted code. Both bash and
// PowerShell treat # as a comment. Control characters are dropped.
func (Adapter) TypeMessage(text string) []byte {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	if !strings.Contains(text, "\n") {
		return []byte(text + "\r")
	}
	var b strings.Builder
	for _, line := range strings.Split(text, "\n") {
		b.WriteString("# ")
		b.WriteString(strings.Map(func(r rune) rune {
			if r < ' ' && r != '\t' || r == 0x7f {
				return -1
			}
			return r
		}, line))
		b.WriteByte('\r')
	}
	return []byte(b.String())
}

// Interrupt sends Ctrl+C.
func (Adapter) Interrupt() []byte { return []byte("\x03") }

// SideQuestion is unsupported.
func (Adapter) SideQuestion(string) ([]byte, bool) { return nil, false }
