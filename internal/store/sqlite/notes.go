package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

const noteCols = `id, session_id, number, title, body, severity, refs, dismissed, at`

func scanNote(r rowScanner) (core.Note, error) {
	var n core.Note
	var refs string
	var dismissed int
	var at int64
	if err := r.Scan(&n.ID, &n.SessionID, &n.Number, &n.Title, &n.Body, &n.Severity, &refs, &dismissed, &at); err != nil {
		return core.Note{}, err
	}
	var err error
	if n.Refs, err = decodeStrings(refs); err != nil {
		return core.Note{}, err
	}
	n.Dismissed = dismissed != 0
	n.At = fromNanos(at)
	return n, nil
}

// ListNotes returns a session's notes ordered by Number ascending.
func (s *Store) ListNotes(ctx context.Context, id core.SessionID) ([]core.Note, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+noteCols+` FROM notes WHERE session_id = ? ORDER BY number`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list notes: %w", err)
	}
	return collect(rows, scanNote, "notes")
}

// DismissNote marks a note dismissed. It returns store.ErrNotFound if the
// note does not exist; dismissing twice is not an error.
func (s *Store) DismissNote(ctx context.Context, noteID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE notes SET dismissed = 1 WHERE id = ?`, noteID)
	if err != nil {
		return fmt.Errorf("sqlite: dismiss note: %w", err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return store.ErrNotFound
	}
	return nil
}
