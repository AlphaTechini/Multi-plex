package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

// DeleteSession removes a session with its summaries, notes, comments and
// checkpoints in one transaction. The approvals audit log is kept (it is
// append-only). Deleting a missing session is not an error.
func (s *Store) DeleteSession(ctx context.Context, id core.SessionID) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("sqlite: delete session: %w", err)
	}
	defer tx.Rollback()
	for _, table := range []string{"summaries", "notes", "comments", "checkpoints"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE session_id = ?`, id); err != nil {
			return fmt.Errorf("sqlite: delete session %s: %w", table, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id); err != nil {
		return fmt.Errorf("sqlite: delete session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("sqlite: delete session: %w", err)
	}
	return nil
}
