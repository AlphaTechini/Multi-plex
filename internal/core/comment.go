package core

import "time"

// CommentStatus is where a review comment is in its life.
type CommentStatus string

// Comment statuses.
const (
	CommentDraft    CommentStatus = "draft"
	CommentQueued   CommentStatus = "queued"
	CommentSent     CommentStatus = "sent"
	CommentOutdated CommentStatus = "outdated"
)

// Comment is a review comment anchored to diff lines. See docs/07-diff-viewer.md.
type Comment struct {
	ID         string        `json:"id"`
	SessionID  SessionID     `json:"session_id"`
	Path       string        `json:"path"` // relative to the workspace root
	NewStart   int           `json:"new_start"`
	NewEnd     int           `json:"new_end"`
	OldStart   int           `json:"old_start,omitempty"`
	OldEnd     int           `json:"old_end,omitempty"`
	BaseCommit string        `json:"base_commit"`
	HeadCommit string        `json:"head_commit"`
	Snippet    string        `json:"snippet"` // the exact selected lines
	Context    string        `json:"context"` // a few lines around them
	Language   string        `json:"language,omitempty"`
	Body       string        `json:"body"`
	Status     CommentStatus `json:"status"`
	Changed    bool          `json:"changed"` // lines changed since sent
	At         time.Time     `json:"at"`
}
