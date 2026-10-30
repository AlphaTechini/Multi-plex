// Package backend is the neutral layer between the TUI and the daemon.
// Views depend only on the Backend interface; the real implementation wraps
// internal/client, and a fake (backend/fake) drives tests and previews.
package backend

import (
	"context"
	"encoding/json"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/protocol"
	"github.com/alphatechini/multi-plex/internal/store"
)

// Event is one event pushed by the daemon.
type Event struct {
	Topic string
	Data  json.RawMessage
}

// Backend is everything the TUI asks of the daemon. Every method may block
// on I/O, so the UI calls them only from tea.Cmd functions, never from
// Update or View.
type Backend interface {
	Events() <-chan Event

	OpenWorkspace(ctx context.Context, dir string) (core.Workspace, error)
	Sessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error)
	Session(ctx context.Context, id core.SessionID) (protocol.SessionDetail, error)
	CreateSession(ctx context.Context, p protocol.SessionCreateParams) (core.Session, error)
	StopSession(ctx context.Context, id core.SessionID) error
	RestartSession(ctx context.Context, id core.SessionID) (core.Session, error)
	RemoveSession(ctx context.Context, id core.SessionID) error
	MarkSeen(ctx context.Context, id core.SessionID) error
	SetFollow(ctx context.Context, id core.SessionID, on bool) error
	Message(ctx context.Context, id core.SessionID, text string, interrupt bool) error
	Interrupt(ctx context.Context, id core.SessionID) error

	OpenShell(ctx context.Context, id core.SessionID, cols, rows int) (core.Pane, error)
	ClosePane(ctx context.Context, id core.PaneID) error
	PaneInput(ctx context.Context, id core.PaneID, data []byte) error
	PaneResize(ctx context.Context, id core.PaneID, cols, rows int) error
	PaneSnapshot(ctx context.Context, id core.PaneID) (protocol.PaneFrame, error)

	Diff(ctx context.Context, id core.SessionID) (protocol.DiffResult, error)
	ReadFile(ctx context.Context, id core.SessionID, path, rev string) (protocol.FileReadResult, error)

	PutComment(ctx context.Context, c core.Comment) (core.Comment, error)
	Comments(ctx context.Context, id core.SessionID) ([]core.Comment, error)
	DeleteComment(ctx context.Context, commentID string) error
	SendComments(ctx context.Context, id core.SessionID) (protocol.CommentSendResult, error)

	Notes(ctx context.Context, id core.SessionID) ([]core.Note, error)
	ActOnNote(ctx context.Context, p protocol.NoteActParams) error

	PendingApprovals(ctx context.Context) ([]core.ApprovalRequest, error)
	RespondApproval(ctx context.Context, p protocol.ApprovalRespondParams) (core.ApprovalDecision, error)
	ApprovalAudit(ctx context.Context, id core.SessionID) ([]store.AuditEntry, error)

	Checkpoints(ctx context.Context, id core.SessionID) ([]core.Checkpoint, error)
	RestoreCheckpoint(ctx context.Context, id core.SessionID, n int) error

	Specs(ctx context.Context, ws core.WorkspaceID) ([]protocol.SpecInfo, error)
	Spec(ctx context.Context, ws core.WorkspaceID, slug string) (protocol.SpecDoc, error)

	Close() error
}
