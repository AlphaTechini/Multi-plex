package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

const commentCols = `id, session_id, path, new_start, new_end, old_start, old_end, base_commit, head_commit,
snippet, context, language, body, status, changed, at`

func scanComment(r rowScanner) (core.Comment, error) {
	var c core.Comment
	var changed int
	var at int64
	err := r.Scan(&c.ID, &c.SessionID, &c.Path, &c.NewStart, &c.NewEnd, &c.OldStart, &c.OldEnd, &c.BaseCommit,
		&c.HeadCommit, &c.Snippet, &c.Context, &c.Language, &c.Body, &c.Status, &changed, &at)
	if err != nil {
		return core.Comment{}, err
	}
	c.Changed = changed != 0
	c.At = fromNanos(at)
	return c, nil
}

// PutComment inserts or replaces a comment by ID.
func (s *Store) PutComment(ctx context.Context, c core.Comment) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO comments (`+commentCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
    session_id = excluded.session_id, path = excluded.path, new_start = excluded.new_start,
    new_end = excluded.new_end, old_start = excluded.old_start, old_end = excluded.old_end,
    base_commit = excluded.base_commit, head_commit = excluded.head_commit, snippet = excluded.snippet,
    context = excluded.context, language = excluded.language, body = excluded.body,
    status = excluded.status, changed = excluded.changed, at = excluded.at`,
		c.ID, c.SessionID, c.Path, c.NewStart, c.NewEnd, c.OldStart, c.OldEnd, c.BaseCommit, c.HeadCommit,
		c.Snippet, c.Context, c.Language, c.Body, c.Status, b2i(c.Changed), nanos(c.At))
	if err != nil {
		return fmt.Errorf("sqlite: put comment: %w", err)
	}
	return nil
}

// ListComments returns a session's comments ordered by Path, then NewStart,
// then ID.
func (s *Store) ListComments(ctx context.Context, id core.SessionID) ([]core.Comment, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+commentCols+` FROM comments WHERE session_id = ? ORDER BY path, new_start, id`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list comments: %w", err)
	}
	return collect(rows, scanComment, "comments")
}

// DeleteComment removes a comment. Deleting a missing comment is not an error.
func (s *Store) DeleteComment(ctx context.Context, commentID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, commentID); err != nil {
		return fmt.Errorf("sqlite: delete comment: %w", err)
	}
	return nil
}
