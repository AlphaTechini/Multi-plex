package core

import "time"

// Session is one agent working on one task, in one worktree, on one branch.
type Session struct {
	ID           SessionID    `json:"id"`
	WorkspaceID  WorkspaceID  `json:"workspace_id"`
	Slug         string       `json:"slug"`
	Task         string       `json:"task"`
	Role         string       `json:"role"`
	Agent        string       `json:"agent"` // adapter name, e.g. "claude-code"
	BaseBranch   string       `json:"base_branch"`
	Branch       string       `json:"branch"`
	WorktreePath string       `json:"worktree_path"`
	State        SessionState `json:"state"`
	StateDetail  string       `json:"state_detail,omitempty"` // e.g. "waiting for approval"
	AgentPane    PaneID       `json:"agent_pane,omitempty"`
	Unseen       bool         `json:"unseen"` // done and not viewed yet
	Follow       bool         `json:"follow"` // follow mode on
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}
