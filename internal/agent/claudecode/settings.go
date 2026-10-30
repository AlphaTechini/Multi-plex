package claudecode

import (
	"encoding/json"
	"strings"
)

// Hook timeouts in seconds. PreToolUse waits for a human-present approval
// (a biometric prompt can take a while); the rest only report state.
const (
	preToolTimeout   = 600
	otherHookTimeout = 30
)

// hookSpec describes one hook registration: Claude Code's event name, the
// neutral name passed to `mpx hook`, and whether it carries a tool matcher.
type hookSpec struct {
	claudeEvent string
	mpxEvent    string
	toolMatcher bool
	gate        bool // fail closed: a hook failure blocks the tool call
}

// hookSpecs lists the hooks Multiplex registers, in output order.
var hookSpecs = []hookSpec{
	{"PreToolUse", "pre-tool", true, true},
	{"PostToolUse", "post-tool", true, false},
	{"PermissionRequest", "permission-request", true, false},
	{"Notification", "notification", false, false},
	{"Stop", "stop", false, false},
	{"UserPromptSubmit", "prompt-submit", false, false},
	{"SessionStart", "session-start", false, false},
	{"SessionEnd", "session-end", false, false},
}

// settingsFile is the JSON passed to `claude --settings`.
type settingsFile struct {
	// DisableAllHooks is written explicitly as false: a project's
	// .claude/settings.json could otherwise switch our hooks off, and
	// --settings outranks project and local settings (docs/04-safety.md).
	DisableAllHooks bool                   `json:"disableAllHooks"`
	Hooks           map[string][]hookGroup `json:"hooks"`
	Sandbox         *sandboxSettings       `json:"sandbox,omitempty"`
}

// sandboxSettings is reserved for Claude Code's own sandbox.
//
// TODO(CC-11, docs/04-safety.md "Using the agent CLI's own sandbox"): on
// macOS and Linux turn the sandbox on with the worktree writable and
// spec.DenyRead on its deny-read list. The exact settings keys are not yet
// verified against Claude Code (the research notes do not cover them), so
// nothing is written. Fill this struct and buildSettings once CC-11 records
// the keys; until then the hook layer and Multiplex's own safety layers are
// the enforcement.
type sandboxSettings struct{}

// hookGroup is a matcher group: handlers that run for matching tools.
type hookGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []hookHandler `json:"hooks"`
}

// hookHandler is one command handler in exec form (no shell on any OS).
type hookHandler struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Timeout int      `json:"timeout"`
	// OnFailure "block" (Claude Code 2.1.295 and later) turns a crash, a
	// timeout or invalid output of a gating hook into a block: the tool call
	// fails closed (docs/04-safety.md).
	OnFailure string `json:"onFailure,omitempty"`
}

// buildSettings returns the contents of settings.json for a session. The
// matcher "*" matches every tool, including Bash and PowerShell (Windows).
func buildSettings(mpxPath string) ([]byte, error) {
	s := settingsFile{Hooks: make(map[string][]hookGroup, len(hookSpecs))}
	for _, h := range hookSpecs {
		handler := hookHandler{
			Type:    "command",
			Command: mpxPath,
			Args:    []string{"hook", h.mpxEvent},
			Timeout: otherHookTimeout,
		}
		if h.gate {
			handler.Timeout = preToolTimeout
			handler.OnFailure = "block"
		}
		group := hookGroup{Hooks: []hookHandler{handler}}
		if h.toolMatcher {
			group.Matcher = "*"
		}
		s.Hooks[h.claudeEvent] = []hookGroup{group}
	}
	return marshalIndent(s)
}

// mcpConfig is the JSON passed to `claude --mcp-config`.
type mcpConfig struct {
	Servers map[string]mcpServer `json:"mcpServers"`
}

type mcpServer struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// buildMCPConfig registers the "multiplex" MCP server (`mpx mcp`) for a
// session. Claude Code starts MCP servers with a reduced environment, so the
// variables the server needs are listed explicitly.
func buildMCPConfig(mpxPath, sessionID string, env []string) ([]byte, error) {
	srvEnv := map[string]string{"MPX_SESSION_ID": sessionID}
	for _, kv := range env {
		if k, v, ok := strings.Cut(kv, "="); ok && k == "MPX_SOCKET" && v != "" {
			srvEnv[k] = v
		}
	}
	return marshalIndent(mcpConfig{Servers: map[string]mcpServer{
		"multiplex": {Type: "stdio", Command: mpxPath, Args: []string{"mcp"}, Env: srvEnv},
	}})
}

// marshalIndent encodes v as indented JSON with a trailing newline and
// without HTML escaping (paths and flags stay readable).
func marshalIndent(v any) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}
