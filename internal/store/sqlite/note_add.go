package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

// AddNote stores n with the next per-session Number (1, 2, 3, ...), generating
// an ID when empty, and returns the stored note. Numbering is atomic: the
// transaction starts with BEGIN IMMEDIATE (see dsnQuery), so concurrent
// callers are serialised and never see the same MAX(number).
func (s *Store) AddNote(ctx context.Context, n core.Note) (core.Note, error) {
	if n.ID == "" {
		n.ID = core.NewID("note")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return core.Note{}, fmt.Errorf("sqlite: add note: %w", err)
	}
	defer tx.Rollback()

	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(number), 0) + 1 FROM notes WHERE session_id = ?`, n.SessionID).Scan(&n.Number); err != nil {
		return core.Note{}, fmt.Errorf("sqlite: add note: next number: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO notes (`+noteCols+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		n.ID, n.SessionID, n.Number, n.Title, n.Body, n.Severity, encodeStrings(n.Refs),
		b2i(n.Dismissed), nanos(n.At)); err != nil {
		return core.Note{}, fmt.Errorf("sqlite: add note: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return core.Note{}, fmt.Errorf("sqlite: add note: %w", err)
	}
	n.At = n.At.Round(0).UTC()
	if n.Refs != nil {
		n.Refs = append([]string{}, n.Refs...)
	}
	return n, nil
}
