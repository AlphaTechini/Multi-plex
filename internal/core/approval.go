package core

import "time"

// ApprovalKind is the kind of gated action.
type ApprovalKind string

// Approval kinds.
const (
	ApprovalCommand   ApprovalKind = "command"
	ApprovalFileWrite ApprovalKind = "file-write"
	ApprovalFileRead  ApprovalKind = "file-read"
	ApprovalPush      ApprovalKind = "push"
)

// ApprovalRequest is a gated action waiting for a human-present approval.
type ApprovalRequest struct {
	ID        string       `json:"id"`
	SessionID SessionID    `json:"session_id"`
	Kind      ApprovalKind `json:"kind"`
	Tool      string       `json:"tool,omitempty"`
	Command   string       `json:"command,omitempty"`
	Cwd       string       `json:"cwd"`
	Paths     []string     `json:"paths,omitempty"`
	Rule      string       `json:"rule"`  // which rule gated it
	Layer     string       `json:"layer"` // which safety layer
	At        time.Time    `json:"at"`
}

// ApprovalOutcome is the result of an approval request.
type ApprovalOutcome string

// Approval outcomes.
const (
	Approved ApprovalOutcome = "approve"
	Denied   ApprovalOutcome = "deny"
)

// ApprovalDecision records how a request was decided, for the audit log.
type ApprovalDecision struct {
	RequestID string          `json:"request_id"`
	Outcome   ApprovalOutcome `json:"outcome"`
	Message   string          `json:"message,omitempty"` // sent to the agent on deny
	Method    string          `json:"method"`            // "windows-hello", "polkit", "typed-code", ...
	At        time.Time       `json:"at"`
}
