package protocol

import (
	"github.com/alphatechini/multi-plex/internal/agent"
	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/vcs"
)

// Request methods. Each lists its params and result types.
const (
	MethodHello     = "hello"     // HelloParams → HelloResult
	MethodSubscribe = "subscribe" // SubscribeParams → Empty; events follow
	MethodShutdown  = "daemon.shutdown"

	MethodWorkspaceOpen = "workspace.open" // WorkspaceOpenParams → core.Workspace
	MethodWorkspaceList = "workspace.list" // Empty → []core.Workspace

	MethodSessionCreate    = "session.create"    // SessionCreateParams → core.Session
	MethodSessionList      = "session.list"      // SessionListParams → []core.Session
	MethodSessionGet       = "session.get"       // SessionRef → SessionDetail
	MethodSessionStop      = "session.stop"      // SessionRef → Empty (kills the agent, keeps worktree)
	MethodSessionRestart   = "session.restart"   // SessionRef → core.Session
	MethodSessionRemove    = "session.remove"    // SessionRef → Empty (removes worktree and branch)
	MethodSessionSeen      = "session.seen"      // SessionRef → Empty (clears Unseen)
	MethodSessionFollow    = "session.follow"    // SessionFollowParams → Empty
	MethodSessionMessage   = "session.message"   // SessionMessageParams → Empty (steer)
	MethodSessionInterrupt = "session.interrupt" // SessionRef → Empty

	MethodPaneOpen     = "pane.open"     // PaneOpenParams → core.Pane (a shell in the worktree)
	MethodPaneClose    = "pane.close"    // PaneRef → Empty
	MethodPaneInput    = "pane.input"    // PaneInputParams → Empty
	MethodPaneResize   = "pane.resize"   // PaneResizeParams → Empty
	MethodPaneSnapshot = "pane.snapshot" // PaneRef → PaneFrame (full screen)

	MethodDiffGet  = "diff.get"  // SessionRef → DiffResult
	MethodFileRead = "file.read" // FileReadParams → FileReadResult

	MethodCommentPut    = "comment.put"    // core.Comment → core.Comment (ID assigned if empty)
	MethodCommentList   = "comment.list"   // SessionRef → []core.Comment
	MethodCommentDelete = "comment.delete" // IDRef → Empty
	MethodCommentSend   = "comment.send"   // SessionRef → CommentSendResult (sends queued comments)

	MethodNoteList = "note.list" // SessionRef → []core.Note
	MethodNoteAct  = "note.act"  // NoteActParams → Empty

	MethodSummaryGet = "summary.get" // SessionRef → core.Summary

	MethodApprovalList    = "approval.list"    // Empty → []core.ApprovalRequest (pending)
	MethodApprovalRespond = "approval.respond" // ApprovalRespondParams → core.ApprovalDecision
	MethodApprovalAudit   = "approval.audit"   // SessionRef → []store.AuditEntry

	MethodCheckpointList    = "checkpoint.list"    // SessionRef → []core.Checkpoint
	MethodCheckpointRestore = "checkpoint.restore" // CheckpointRestoreParams → Empty

	MethodAgentList = "agent.list" // Empty → []AgentInfo (adapters the daemon knows)
	MethodRoleList  = "role.list"  // WorkspaceRef → []RoleInfo (roles visible to the workspace)

	MethodSpecList = "spec.list" // WorkspaceRef → []SpecInfo
	MethodSpecGet  = "spec.get"  // SpecGetParams → SpecDoc

	MethodHook = "hook.event" // HookParams → agent.HookResponse (used by `mpx hook`)
	MethodMCP  = "mcp.call"   // MCPParams → MCPResult (used by `mpx mcp`)
)

// Empty is an empty params or result object.
type Empty struct{}

// HelloParams opens a connection.
type HelloParams struct {
	Client   string `json:"client"` // "tui", "hook", "mcp", ...
	Protocol int    `json:"protocol"`
}

// HelloResult answers hello.
type HelloResult struct {
	Protocol int    `json:"protocol"`
	Version  string `json:"version"`
	PID      int    `json:"pid"`
	Approver string `json:"approver"` // name of the active approval method
}

// SubscribeParams selects event topics. Empty Topics means all.
type SubscribeParams struct {
	Topics []string `json:"topics,omitempty"`
}

// WorkspaceOpenParams opens (or finds) the workspace containing Dir.
type WorkspaceOpenParams struct {
	Dir string `json:"dir"`
}

// WorkspaceRef names a workspace.
type WorkspaceRef struct {
	WorkspaceID core.WorkspaceID `json:"workspace_id"`
}

// SessionCreateParams creates a session.
type SessionCreateParams struct {
	WorkspaceID core.WorkspaceID `json:"workspace_id"`
	Task        string           `json:"task"`
	Name        string           `json:"name,omitempty"`        // slug source; derived from Task if empty
	Role        string           `json:"role,omitempty"`        // default "implementer"
	Agent       string           `json:"agent,omitempty"`       // default "claude-code"
	BaseBranch  string           `json:"base_branch,omitempty"` // default: current branch
	Cols        int              `json:"cols,omitempty"`        // initial agent pane size
	Rows        int              `json:"rows,omitempty"`
}

// SessionListParams filters sessions.
type SessionListParams struct {
	WorkspaceID core.WorkspaceID `json:"workspace_id,omitempty"`
}

// SessionRef names a session.
type SessionRef struct {
	SessionID core.SessionID `json:"session_id"`
}

// IDRef names a record by ID.
type IDRef struct {
	ID string `json:"id"`
}

// SessionDetail is everything the Overview needs for one session.
type SessionDetail struct {
	Session     core.Session     `json:"session"`
	Summary     *core.Summary    `json:"summary,omitempty"`
	Notes       []core.Note      `json:"notes"`
	Conflicts   []core.Conflict  `json:"conflicts"`
	Changes     []vcs.FileChange `json:"changes"`
	Checkpoints int              `json:"checkpoints"`
	Approvals   int              `json:"approvals_last_hour"`
	LastMessage string           `json:"last_message,omitempty"` // fallback when no summary
	Turn        int              `json:"turn"`                   // prompts submitted to the agent so far
	Reports     int              `json:"reports"`                // progress reports (summaries) so far
}

// SessionFollowParams toggles follow mode.
type SessionFollowParams struct {
	SessionID core.SessionID `json:"session_id"`
	On        bool           `json:"on"`
}

// SessionMessageParams types a message into a session's agent.
type SessionMessageParams struct {
	SessionID core.SessionID `json:"session_id"`
	Text      string         `json:"text"`
	Interrupt bool           `json:"interrupt,omitempty"` // interrupt first
}

// PaneOpenParams opens a shell pane in a session's worktree.
type PaneOpenParams struct {
	SessionID core.SessionID `json:"session_id"`
	Cols      int            `json:"cols"`
	Rows      int            `json:"rows"`
}

// PaneRef names a pane.
type PaneRef struct {
	PaneID core.PaneID `json:"pane_id"`
}

// PaneInputParams sends input bytes to a pane.
type PaneInputParams struct {
	PaneID core.PaneID `json:"pane_id"`
	Data   []byte      `json:"data"` // base64 in JSON
}

// PaneResizeParams resizes a pane.
type PaneResizeParams struct {
	PaneID core.PaneID `json:"pane_id"`
	Cols   int         `json:"cols"`
	Rows   int         `json:"rows"`
}

// DiffResult is a session's diff against its base.
type DiffResult struct {
	Base    string           `json:"base"` // base commit
	Head    string           `json:"head"` // HEAD commit of the worktree
	Raw     string           `json:"raw"`  // unified diff text
	Changes []vcs.FileChange `json:"changes"`
}

// FileReadParams reads a file in a session's worktree.
type FileReadParams struct {
	SessionID core.SessionID `json:"session_id"`
	Path      string         `json:"path"`          // relative to the worktree root
	Rev       string         `json:"rev,omitempty"` // "" = working tree
}

// FileReadResult is a file's content.
type FileReadResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Binary  bool   `json:"binary"`
}

// CommentSendResult reports what was sent.
type CommentSendResult struct {
	Sent    int    `json:"sent"`
	Message string `json:"message"` // the exact text typed into the agent
}

// NoteActParams acts on an insight card.
type NoteActParams struct {
	NoteID string          `json:"note_id"`
	Action core.NoteAction `json:"action"`
	Text   string          `json:"text,omitempty"` // question or steer message
}

// ApprovalRespondParams answers a pending approval.
type ApprovalRespondParams struct {
	RequestID string               `json:"request_id"`
	Outcome   core.ApprovalOutcome `json:"outcome"`
	Message   string               `json:"message,omitempty"` // deny with a message to the agent
	Code      string               `json:"code,omitempty"`    // for the typed-code method
}

// CheckpointRestoreParams rolls a session back.
type CheckpointRestoreParams struct {
	SessionID core.SessionID `json:"session_id"`
	N         int            `json:"n"`
}

// SpecInfo summarises a spec for the sidebar.
type SpecInfo struct {
	Slug          string `json:"slug"`
	Title         string `json:"title"`
	TasksDone     int    `json:"tasks_done"`
	TasksTotal    int    `json:"tasks_total"`
	CriteriaTotal int    `json:"criteria_total"`
	// CriteriaVerified counts distinct criteria that agents recorded as met
	// with criterion_verify (verification.md, in any session's worktree).
	CriteriaVerified int `json:"criteria_verified"`
}

// AgentInfo describes one agent adapter the daemon can run.
type AgentInfo struct {
	Name        string `json:"name"`         // registry name, the value of SessionCreateParams.Agent
	DisplayName string `json:"display_name"` // e.g. "Claude Code"
	Available   bool   `json:"available"`    // the agent's program was found on PATH
	Detail      string `json:"detail"`       // where it was found, or how to install it
	Default     bool   `json:"default"`      // used when a session names no agent
}

// RoleInfo describes one role visible to a workspace.
type RoleInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`  // "builtin", "global" or "workspace"
	Default     bool   `json:"default"` // used when a session names no role
}

// SpecGetParams reads one spec.
type SpecGetParams struct {
	WorkspaceID core.WorkspaceID `json:"workspace_id"`
	Slug        string           `json:"slug"`
}

// SpecDoc is a spec's three files.
type SpecDoc struct {
	Slug         string `json:"slug"`
	Requirements string `json:"requirements"`
	Design       string `json:"design"`
	Tasks        string `json:"tasks"`
}

// HookParams carries one agent CLI hook call from `mpx hook`.
type HookParams struct {
	SessionID core.SessionID  `json:"session_id"` // from MPX_SESSION_ID
	Adapter   string          `json:"adapter"`
	Event     agent.HookEvent `json:"event"`
}

// MCPParams carries one MCP tool call from `mpx mcp`.
type MCPParams struct {
	SessionID core.SessionID `json:"session_id"` // from MPX_SESSION_ID
	Tool      string         `json:"tool"`
	Args      []byte         `json:"args"` // the tool's JSON arguments
}

// MCPResult is a tool result as text.
type MCPResult struct {
	Text    string `json:"text"`
	IsError bool   `json:"is_error,omitempty"`
}
