package protocol

import "github.com/alphatechini/multi-plex/internal/core"

// Event topics pushed by the daemon after subscribe.
const (
	TopicSession         = "session.updated"   // core.Session
	TopicSessionRemoved  = "session.removed"   // SessionRef
	TopicPaneFrame       = "pane.frame"        // PaneFrame (changed rows only)
	TopicPaneExited      = "pane.exited"       // core.Pane
	TopicNote            = "note.added"        // core.Note
	TopicSummary         = "summary.updated"   // core.Summary
	TopicApprovalPending = "approval.pending"  // ApprovalPending
	TopicApprovalDone    = "approval.resolved" // core.ApprovalDecision
	TopicDiffChanged     = "diff.changed"      // SessionRef
	TopicEdit            = "edit"              // core.EditEvent (follow mode)
	TopicConflicts       = "conflicts.updated" // ConflictsEvent
	TopicToast           = "toast"             // Toast
)

// Row is one changed screen row, rendered with ANSI SGR styling and exactly
// Cols cells wide.
type Row struct {
	Y    int    `json:"y"`
	Text string `json:"s"`
}

// PaneFrame is a screen update for a pane. A full frame carries every row.
type PaneFrame struct {
	PaneID        core.PaneID `json:"pane_id"`
	Cols          int         `json:"cols"`
	Rows          int         `json:"rows"`
	Full          bool        `json:"full"`
	Changed       []Row       `json:"changed"`
	CursorX       int         `json:"cx"`
	CursorY       int         `json:"cy"`
	CursorVisible bool        `json:"cv"`
	Title         string      `json:"title,omitempty"`
}

// ApprovalPending is pushed when an action is gated.
type ApprovalPending struct {
	Request core.ApprovalRequest `json:"request"`
	Method  string               `json:"method"`         // verifier that will be used
	Code    string               `json:"code,omitempty"` // one-time code for the typed-code method (shown only in the UI)
}

// ConflictsEvent is pushed when conflict prediction for a session changes.
type ConflictsEvent struct {
	SessionID core.SessionID  `json:"session_id"`
	Conflicts []core.Conflict `json:"conflicts"`
}

// Toast is a short message for the UI.
type Toast struct {
	Level string `json:"level"` // info, success, warning, error
	Text  string `json:"text"`
}
