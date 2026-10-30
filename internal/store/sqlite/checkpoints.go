package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
)

const checkpointCols = `session_id, n, ref, commit_id, reason, at`

func scanCheckpoint(r rowScanner) (core.Checkpoint, error) {
	var c core.Checkpoint
	var at int64
	if err := r.Scan(&c.SessionID, &c.N, &c.Ref, &c.Commit, &c.Reason, &at); err != nil {
		return core.Checkpoint{}, err
	}
	c.At = fromNanos(at)
	return c, nil
}

// AddCheckpoint stores a checkpoint; one with the same session and N is
// replaced.
func (s *Store) AddCheckpoint(ctx context.Context, c core.Checkpoint) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO checkpoints (`+checkpointCols+`) VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id, n) DO UPDATE SET
    ref = excluded.ref, commit_id = excluded.commit_id, reason = excluded.reason, at = excluded.at`,
		c.SessionID, c.N, c.Ref, c.Commit, c.Reason, nanos(c.At))
	if err != nil {
		return fmt.Errorf("sqlite: add checkpoint: %w", err)
	}
	return nil
}

// ListCheckpoints returns a session's checkpoints ordered by N ascending.
func (s *Store) ListCheckpoints(ctx context.Context, id core.SessionID) ([]core.Checkpoint, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+checkpointCols+` FROM checkpoints WHERE session_id = ? ORDER BY n`, id)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list checkpoints: %w", err)
	}
	return collect(rows, scanCheckpoint, "checkpoints")
}
