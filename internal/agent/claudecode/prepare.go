package claudecode

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alphatechini/multi-plex/internal/agent"
)

// lookPath finds executables; tests replace it.
var lookPath = exec.LookPath

// File names Prepare writes into spec.ConfigDir.
const (
	settingsName     = "settings.json"
	mcpName          = "mcp.json"
	systemPromptName = "system-prompt.md"
)

// Prepare writes the session's Claude Code configuration into spec.ConfigDir
// and returns the command that starts it:
//
//	settings.json      our hooks only, plus "disableAllHooks": false
//	mcp.json           the "multiplex" MCP server
//	system-prompt.md   spec.Instructions and spec.RolePrompt (when non-empty)
//
// The files are daemon-owned: ConfigDir must be outside the worktree, or the
// agent could edit its own hooks. The prompt goes through a file rather than
// the command line so multi-line text survives every shell and Windows
// command-line quoting layer unchanged.
func (Adapter) Prepare(spec agent.LaunchSpec) (agent.Launch, error) {
	if spec.ConfigDir == "" {
		return agent.Launch{}, errors.New("claudecode: ConfigDir is required")
	}
	if !filepath.IsAbs(spec.MpxPath) {
		return agent.Launch{}, fmt.Errorf("claudecode: MpxPath must be an absolute path, got %q", spec.MpxPath)
	}
	if insideDir(spec.WorktreePath, spec.ConfigDir) {
		return agent.Launch{}, fmt.Errorf("claudecode: ConfigDir %q is inside the worktree; the agent could rewrite its own hooks", spec.ConfigDir)
	}
	claude, err := findClaude()
	if err != nil {
		return agent.Launch{}, err
	}

	if err := os.MkdirAll(spec.ConfigDir, 0o700); err != nil {
		return agent.Launch{}, fmt.Errorf("claudecode: create config dir: %w", err)
	}
	sessionID := string(spec.Session.ID)

	settings, err := buildSettings(spec)
	if err != nil {
		return agent.Launch{}, err
	}
	mcp, err := buildMCPConfig(spec.MpxPath, sessionID, spec.Env)
	if err != nil {
		return agent.Launch{}, err
	}
	settingsPath := filepath.Join(spec.ConfigDir, settingsName)
	mcpPath := filepath.Join(spec.ConfigDir, mcpName)
	if err := writeFile(settingsPath, settings); err != nil {
		return agent.Launch{}, err
	}
	if err := writeFile(mcpPath, mcp); err != nil {
		return agent.Launch{}, err
	}

	argv := []string{claude, "--settings", settingsPath, "--mcp-config", mcpPath}
	if prompt := systemPrompt(spec); prompt != "" {
		promptPath := filepath.Join(spec.ConfigDir, systemPromptName)
		if err := writeFile(promptPath, []byte(prompt)); err != nil {
			return agent.Launch{}, err
		}
		argv = append(argv, "--append-system-prompt-file", promptPath)
	}
	if m := spec.Model; m != "" && m != "default" {
		argv = append(argv, "--model", m)
	}
	// The tool lists and --mcp-config are variadic flags: each value is its
	// own argument, and the prompt follows after "--" so none can swallow it.
	if len(spec.AllowedTools) > 0 {
		argv = append(argv, "--allowedTools")
		argv = append(argv, spec.AllowedTools...)
	}
	if len(spec.DeniedTools) > 0 {
		argv = append(argv, "--disallowedTools")
		argv = append(argv, spec.DeniedTools...)
	}
	if task := strings.TrimSpace(spec.Task); task != "" {
		argv = append(argv, "--", initialPrompt(task))
	}

	return agent.Launch{
		Argv: argv,
		Env:  withEnv(spec.Env, "MPX_SESSION_ID="+sessionID, "MPX_ADAPTER="+Name),
		Dir:  spec.WorktreePath,
	}, nil
}

// initialPrompt returns the positional prompt for task. The task is passed
// as one argument, so it needs no quoting. A one-word task would be read as a
// Claude Code subcommand ("doctor", "update", ...) even after "--", so it
// gets a short lead-in.
func initialPrompt(task string) string {
	if strings.ContainsAny(task, " \t\r\n") {
		return task
	}
	return "Task: " + task
}

// systemPrompt joins the instructions and the role prompt.
func systemPrompt(spec agent.LaunchSpec) string {
	var parts []string
	for _, p := range []string{spec.Instructions, spec.RolePrompt} {
		if p = strings.TrimSpace(p); p != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n\n")
}

// findClaude locates the claude executable. On Windows it prefers the native
// claude.exe over an npm .cmd shim, which ConPTY cannot start directly.
func findClaude() (string, error) {
	names := []string{"claude"}
	if runtime.GOOS == "windows" {
		names = []string{"claude.exe", "claude.cmd", "claude"}
	}
	for _, n := range names {
		if p, err := lookPath(n); err == nil {
			return p, nil
		}
	}
	return "", errors.New("claudecode: the claude command was not found on PATH (install Claude Code first)")
}

// insideDir reports whether path is dir or lies under it.
func insideDir(dir, path string) bool {
	if dir == "" || path == "" {
		return false
	}
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// writeFile writes data to path atomically with owner-only permissions.
func writeFile(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return fmt.Errorf("claudecode: write %s: %w", filepath.Base(path), err)
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("claudecode: write %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("claudecode: write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// withEnv returns base (or the current environment when nil) with the given
// KEY=value entries set, replacing any entry of the same name.
func withEnv(base []string, extra ...string) []string {
	if base == nil {
		base = os.Environ()
	}
	out := make([]string, 0, len(base)+len(extra))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		dup := false
		for _, e := range extra {
			if ek, _, _ := strings.Cut(e, "="); envKeyEqual(k, ek) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, kv)
		}
	}
	return append(out, extra...)
}

func envKeyEqual(a, b string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}
