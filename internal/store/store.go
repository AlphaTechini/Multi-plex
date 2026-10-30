// Package store is the neutral persistence layer. The daemon depends on the
// Store interface only; SQLite (modernc.org/sqlite) is one implementation in
// a subpackage, and an in-memory one exists for tests.
package store

import (
	"context"
	"errors"

	"github.com/alphatechini/multi-plex/internal/core"
)

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("store: not found")

// Store persists everything that is not in the repository.
// See docs/01-architecture.md "Storage".
//
// Conventions shared by every implementation (checked by storetest):
// a limit <= 0 means no limit; deleting a missing record is not an error,
// but DismissNote on a missing note returns ErrNotFound; ListApprovals
// matches one session exactly; times are returned in UTC.
type Store interface {
	// Workspaces.
	PutWorkspace(ctx context.Context, w core.Workspace) error
	GetWorkspace(ctx context.Context, id core.WorkspaceID) (core.Workspace, error)
	WorkspaceByRoot(ctx context.Context, root string) (core.Workspace, error)
	ListWorkspaces(ctx context.Context) ([]core.Workspace, error)

	// Sessions.
	PutSession(ctx context.Context, s core.Session) error
	GetSession(ctx context.Context, id core.SessionID) (core.Session, error)
	ListSessions(ctx context.Context, ws core.WorkspaceID) ([]core.Session, error) // ws "" lists all
	DeleteSession(ctx context.Context, id core.SessionID) error

	// Summaries: the latest is what Overview shows; history is kept.
	AddSummary(ctx context.Context, s core.Summary) error
	LatestSummary(ctx context.Context, id core.SessionID) (core.Summary, error)
	ListSummaries(ctx context.Context, id core.SessionID, limit int) ([]core.Summary, error)

	// Notes. AddNote assigns Number (next per session) and returns the note.
	AddNote(ctx context.Context, n core.Note) (core.Note, error)
	ListNotes(ctx context.Context, id core.SessionID) ([]core.Note, error)
	DismissNote(ctx context.Context, noteID string) error

	// Review comments.
	PutComment(ctx context.Context, c core.Comment) error
	ListComments(ctx context.Context, id core.SessionID) ([]core.Comment, error)
	DeleteComment(ctx context.Context, commentID string) error

	// Checkpoints.
	AddCheckpoint(ctx context.Context, c core.Checkpoint) error
	ListCheckpoints(ctx context.Context, id core.SessionID) ([]core.Checkpoint, error)

	// Audit log of approvals. Append-only.
	LogApproval(ctx context.Context, r core.ApprovalRequest, d core.ApprovalDecision) error
	ListApprovals(ctx context.Context, id core.SessionID, limit int) ([]AuditEntry, error)

	Close() error
}

// AuditEntry is one row of the approvals audit log.
type AuditEntry struct {
	Request  core.ApprovalRequest  `json:"request"`
	Decision core.ApprovalDecision `json:"decision"`
}
