// Package agent is the neutral layer between Multiplex and agent CLIs
// (docs/01-architecture.md "Integration points"). The core never depends on
// a specific CLI: each CLI is an Adapter in its own subpackage (claudecode,
// shell, later opencode, codex, native) registered by name.
package agent

import "github.com/alphatechini/multi-plex/internal/core"

// Capabilities declares what an adapter supports.
type Capabilities struct {
	Hooks         bool // can gate tool calls and report state through hooks
	MCP           bool // can call the mpx MCP server
	Sandbox       bool // has its own OS sandbox on this platform
	SideQuestions bool // supports side questions that do not change history (/btw)
	Roles         bool // can apply role prompts and tool lists
}

// LaunchSpec is everything an adapter needs to start a session's agent.
type LaunchSpec struct {
	Session       core.Session
	WorkspaceRoot string
	WorktreePath  string
	ConfigDir     string   // per-session directory the adapter may write config into
	MpxPath       string   // absolute path of the mpx binary, for hook and MCP commands
	Env           []string // scrubbed base environment (docs/04-safety.md Layer 3)
	RolePrompt    string   // appended system prompt from the role, may be empty
	AllowedTools  []string
	DeniedTools   []string
	Model         string   // "" or "default" means the CLI's default
	DenyRead      []string // credential stores, for adapters with a sandbox
	Instructions  string   // short instruction telling the agent to use mpx MCP tools
}

// Launch is the resolved command line for a session's agent.
type Launch struct {
	Argv []string
	Env  []string
	Dir  string
}

// HookKind is a neutral hook event kind.
type HookKind string

// Hook kinds.
const (
	HookPreTool      HookKind = "pre-tool"
	HookPostTool     HookKind = "post-tool"
	HookNotification HookKind = "notification"
	HookStop         HookKind = "stop"
	HookPromptSubmit HookKind = "prompt-submit"
	HookSessionStart HookKind = "session-start"
	HookSessionEnd   HookKind = "session-end"
)

// ToolClass is a neutral classification of a tool call.
type ToolClass string

// Tool classes.
const (
	ToolShell ToolClass = "shell" // runs a command
	ToolWrite ToolClass = "write" // creates or edits files
	ToolRead  ToolClass = "read"  // reads files or searches
	ToolOther ToolClass = "other"
)

// HookEvent is an agent CLI hook event translated into neutral terms.
type HookEvent struct {
	Kind      HookKind       `json:"kind"`
	AgentRef  string         `json:"agent_ref,omitempty"` // the CLI's own session id
	Cwd       string         `json:"cwd,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolClass ToolClass      `json:"tool_class,omitempty"`
	Command   string         `json:"command,omitempty"` // for shell tools
	Paths     []string       `json:"paths,omitempty"`   // files a tool reads or writes (as given)
	Edits     []EditRange    `json:"edits,omitempty"`   // for follow mode, post-tool only
	Message   string         `json:"message,omitempty"` // notification text
	ToolInput map[string]any `json:"tool_input,omitempty"`
}

// EditRange is a line range an edit tool touched (1-based, inclusive).
type EditRange struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

// Decision is the gate's answer to a pre-tool hook.
type Decision string

// Decisions.
const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
	Ask   Decision = "ask"
)

// HookResponse is the daemon's answer to a hook event.
type HookResponse struct {
	Decision     Decision       `json:"decision,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	UpdatedInput map[string]any `json:"updated_input,omitempty"`
}

// Adapter knows how to run and talk to one agent CLI.
type Adapter interface {
	// Name is the stable identifier, e.g. "claude-code".
	Name() string
	// DisplayName is shown in the UI, e.g. "Claude Code".
	DisplayName() string
	// Capabilities reports what this adapter supports on this platform.
	Capabilities() Capabilities
	// Prepare writes any config files into spec.ConfigDir (hooks, MCP
	// registration) and returns the command line to run in the PTY.
	Prepare(spec LaunchSpec) (Launch, error)
	// ParseHook translates a raw hook payload for the named CLI event
	// (as passed to `mpx hook <event>`) into a neutral event.
	ParseHook(event string, payload []byte) (HookEvent, error)
	// EncodeHookResponse turns the daemon's answer into what the CLI
	// expects on stdout, plus the process exit code.
	EncodeHookResponse(ev HookEvent, r HookResponse) (stdout []byte, exitCode int)
	// FailClosed returns the stdout and exit code that block the tool call
	// when the daemon cannot be reached.
	FailClosed(reason string) (stdout []byte, exitCode int)
	// TypeMessage returns the bytes that type text into the agent's input
	// and submit it.
	TypeMessage(text string) []byte
	// Interrupt returns the bytes that interrupt the agent's current step.
	Interrupt() []byte
	// SideQuestion returns the bytes that ask a side question without
	// changing the main conversation, or ok=false if unsupported.
	SideQuestion(text string) (b []byte, ok bool)
}
