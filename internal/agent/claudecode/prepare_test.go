package claudecode

import (
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
)

var update = flag.Bool("update", false, "rewrite golden files")

// fakeClaude makes findClaude succeed without Claude Code installed.
func fakeClaude(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude")
	old := lookPath
	lookPath = func(string) (string, error) { return path, nil }
	t.Cleanup(func() { lookPath = old })
	return path
}

// absMpx returns an absolute mpx path valid on this OS.
func absMpx(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "bin", "mpx")
}

func baseSpec(t *testing.T) agent.LaunchSpec {
	t.Helper()
	root := t.TempDir()
	return agent.LaunchSpec{
		Session:       core.Session{ID: "ses_1f3a9c0b2d4e6f70"},
		WorkspaceRoot: filepath.Join(root, "repo"),
		WorktreePath:  filepath.Join(root, "repo-wt", "feature"),
		ConfigDir:     filepath.Join(root, "data", "ses_1f3a9c0b2d4e6f70"),
		MpxPath:       absMpx(t),
		Env:           []string{"PATH=/usr/bin", "HOME=/home/u", "MPX_SOCKET=/run/mpx.sock"},
	}
}

// golden compares got with testdata/name. The machine-specific mpx path is
// replaced by {{MPX}} first.
func golden(t *testing.T, name string, got []byte, mpx string) {
	t.Helper()
	enc, _ := json.Marshal(mpx)
	norm := strings.ReplaceAll(string(got), strings.Trim(string(enc), `"`), "{{MPX}}")
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(norm), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if norm != string(want) {
		t.Errorf("%s differs from golden file (run with -update to refresh):\n--- got\n%s\n--- want\n%s", name, norm, want)
	}
}

func TestPrepareWritesConfig(t *testing.T) {
	claude := fakeClaude(t)
	spec := baseSpec(t)
	spec.Instructions = "Use the multiplex MCP tools."
	spec.RolePrompt = "You are a reviewer."
	spec.Model = "opus"
	spec.AllowedTools = []string{"Read", "Bash(git log *)"}
	spec.DeniedTools = []string{"WebFetch"}

	l, err := Adapter{}.Prepare(spec)
	if err != nil {
		t.Fatal(err)
	}

	settingsPath := filepath.Join(spec.ConfigDir, "settings.json")
	mcpPath := filepath.Join(spec.ConfigDir, "mcp.json")
	promptPath := filepath.Join(spec.ConfigDir, "system-prompt.md")
	wantArgv := []string{
		claude,
		"--settings", settingsPath,
		"--mcp-config", mcpPath,
		"--append-system-prompt-file", promptPath,
		"--model", "opus",
		"--allowedTools", "Read", "Bash(git log *)",
		"--disallowedTools", "WebFetch",
	}
	if !slices.Equal(l.Argv, wantArgv) {
		t.Errorf("argv =\n %q\nwant\n %q", l.Argv, wantArgv)
	}
	if l.Dir != spec.WorktreePath {
		t.Errorf("Dir = %q", l.Dir)
	}

	settings, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "settings.golden.json", settings, spec.MpxPath)
	mcp, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "mcp.golden.json", mcp, spec.MpxPath)

	prompt, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(prompt) != "Use the multiplex MCP tools.\n\nYou are a reviewer." {
		t.Errorf("system prompt = %q", prompt)
	}
	if runtime.GOOS != "windows" {
		for _, p := range []string{settingsPath, mcpPath, promptPath} {
			if st, err := os.Stat(p); err != nil || st.Mode().Perm() != 0o600 {
				t.Errorf("%s: mode %v, err %v; want 0600", filepath.Base(p), st.Mode().Perm(), err)
			}
		}
	}
}

// TestSettingsShape checks the properties docs/04-safety.md depends on,
// independent of the golden file.
func TestSettingsShape(t *testing.T) {
	mpx := absMpx(t)
	raw, err := buildSettings(agent.LaunchSpec{MpxPath: mpx})
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		DisableAllHooks *bool `json:"disableAllHooks"`
		Hooks           map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type, Command, OnFailure string
				Args                     []string
				Timeout                  int
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s.DisableAllHooks == nil || *s.DisableAllHooks {
		t.Error(`settings must contain "disableAllHooks": false`)
	}
	for _, ev := range []string{"PreToolUse", "PostToolUse", "PermissionRequest", "Notification", "Stop", "UserPromptSubmit", "SessionStart", "SessionEnd"} {
		groups := s.Hooks[ev]
		if len(groups) != 1 || len(groups[0].Hooks) != 1 {
			t.Fatalf("%s: want exactly one handler, got %+v", ev, groups)
		}
		h := groups[0].Hooks[0]
		if h.Type != "command" || h.Command != mpx || len(h.Args) != 2 || h.Args[0] != "hook" {
			t.Errorf("%s: handler = %+v, want exec form %q hook <event>", ev, h, mpx)
		}
		if ev == "PreToolUse" {
			if h.OnFailure != "block" || h.Timeout != 600 || groups[0].Matcher != "*" {
				t.Errorf("PreToolUse must fail closed with a 600 s timeout on every tool, got %+v matcher %q", h, groups[0].Matcher)
			}
		} else if h.OnFailure == "block" {
			t.Errorf("%s: observation hooks must not block on failure", ev)
		}
	}
	if len(s.Hooks) != 8 {
		t.Errorf("registered %d events, want 8", len(s.Hooks))
	}
	if strings.Contains(string(raw), `"sandbox"`) {
		t.Error("sandbox keys are unverified and must not be written yet (CC-11)")
	}
}

func TestPrepareMinimal(t *testing.T) {
	claude := fakeClaude(t)
	spec := baseSpec(t)
	spec.Model = "default"
	l, err := Adapter{}.Prepare(spec)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{claude, "--settings", filepath.Join(spec.ConfigDir, "settings.json"), "--mcp-config", filepath.Join(spec.ConfigDir, "mcp.json")}
	if !slices.Equal(l.Argv, want) {
		t.Errorf("argv = %q, want %q", l.Argv, want)
	}
	if _, err := os.Stat(filepath.Join(spec.ConfigDir, "system-prompt.md")); err == nil {
		t.Error("no prompt file expected when there is no prompt")
	}
}

func TestPrepareEnv(t *testing.T) {
	fakeClaude(t)
	spec := baseSpec(t)
	spec.Env = append(spec.Env, "MPX_SESSION_ID=stale")
	l, err := Adapter{}.Prepare(spec)
	if err != nil {
		t.Fatal(err)
	}
	count := map[string]int{}
	for _, kv := range l.Env {
		k, _, _ := strings.Cut(kv, "=")
		count[k]++
	}
	for _, kv := range []string{"MPX_SESSION_ID=ses_1f3a9c0b2d4e6f70", "MPX_ADAPTER=claude-code", "MPX_SOCKET=/run/mpx.sock", "PATH=/usr/bin"} {
		if !slices.Contains(l.Env, kv) {
			t.Errorf("env lacks %q: %v", kv, l.Env)
		}
	}
	if count["MPX_SESSION_ID"] != 1 || count["MPX_ADAPTER"] != 1 {
		t.Errorf("duplicate variables in env: %v", l.Env)
	}
}

func TestPrepareErrors(t *testing.T) {
	fakeClaude(t)
	tests := []struct {
		name string
		mod  func(*agent.LaunchSpec)
	}{
		{"no config dir", func(s *agent.LaunchSpec) { s.ConfigDir = "" }},
		{"relative mpx path", func(s *agent.LaunchSpec) { s.MpxPath = "mpx" }},
		{"empty mpx path", func(s *agent.LaunchSpec) { s.MpxPath = "" }},
		{"config dir inside worktree", func(s *agent.LaunchSpec) { s.ConfigDir = filepath.Join(s.WorktreePath, ".mpx") }},
		{"config dir is worktree", func(s *agent.LaunchSpec) { s.ConfigDir = s.WorktreePath }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := baseSpec(t)
			tt.mod(&spec)
			if _, err := (Adapter{}).Prepare(spec); err == nil {
				t.Error("want an error")
			}
		})
	}

	t.Run("claude missing", func(t *testing.T) {
		old := lookPath
		lookPath = func(string) (string, error) { return "", errors.New("not found") }
		defer func() { lookPath = old }()
		if _, err := (Adapter{}).Prepare(baseSpec(t)); err == nil || !strings.Contains(err.Error(), "claude") {
			t.Errorf("err = %v", err)
		}
	})
}

func TestInsideDir(t *testing.T) {
	root := t.TempDir()
	wt := filepath.Join(root, "wt")
	tests := []struct {
		dir, path string
		want      bool
	}{
		{wt, wt, true},
		{wt, filepath.Join(wt, "a", "b"), true},
		{wt, filepath.Join(root, "wt2"), false},
		{wt, filepath.Join(root, "other"), false},
		{wt, root, false},
		{"", wt, false},
	}
	for _, tt := range tests {
		if got := insideDir(tt.dir, tt.path); got != tt.want {
			t.Errorf("insideDir(%q, %q) = %v, want %v", tt.dir, tt.path, got, tt.want)
		}
	}
}

func TestRegisteredAndCapabilities(t *testing.T) {
	a, ok := agent.Get("claude-code")
	if !ok {
		t.Fatal("claude-code adapter not registered")
	}
	if a.DisplayName() != "Claude Code" {
		t.Errorf("DisplayName = %q", a.DisplayName())
	}
	c := a.Capabilities()
	if !c.Hooks || !c.MCP || !c.SideQuestions || !c.Roles || c.Sandbox != (runtime.GOOS != "windows") {
		t.Errorf("Capabilities = %+v", c)
	}
}
