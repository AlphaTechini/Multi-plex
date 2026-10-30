package sqlite

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

const summaryCols = `session_id, done, files_changed, decisions, open_questions, next, at`

func scanSummary(r rowScanner) (core.Summary, error) {
	var s core.Summary
	var done, files, decisions, questions string
	var at int64
	if err := r.Scan(&s.SessionID, &done, &files, &decisions, &questions, &s.Next, &at); err != nil {
		return core.Summary{}, err
	}
	var err error
	if s.Done, err = decodeStrings(done); err != nil {
		return core.Summary{}, err
	}
	if s.FilesChanged, err = decodeStrings(files); err != nil {
		return core.Summary{}, err
	}
	if s.Decisions, err = decodeStrings(decisions); err != nil {
		return core.Summary{}, err
	}
	if s.OpenQuestions, err = decodeStrings(questions); err != nil {
		return core.Summary{}, err
	}
	s.At = fromNanos(at)
	return s, nil
}

// AddSummary appends a summary to the session's history.
func (s *Store) AddSummary(ctx context.Context, x core.Summary) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO summaries (`+summaryCols+`) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		x.SessionID, encodeStrings(x.Done), encodeStrings(x.FilesChanged), encodeStrings(x.Decisions),
		encodeStrings(x.OpenQuestions), x.Next, nanos(x.At))
	if err != nil {
		return fmt.Errorf("sqlite: add summary: %w", err)
	}
	return nil
}

// LatestSummary returns the newest summary of a session or store.ErrNotFound.
func (s *Store) LatestSummary(ctx context.Context, id core.SessionID) (core.Summary, error) {
	got, err := s.ListSummaries(ctx, id, 1)
	if err != nil {
		return core.Summary{}, err
	}
	if len(got) == 0 {
		return core.Summary{}, store.ErrNotFound
	}
	return got[0], nil
}

// ListSummaries returns a session's summaries newest first (by At, later
// insertions first on ties). A limit <= 0 means no limit.
func (s *Store) ListSummaries(ctx context.Context, id core.SessionID, limit int) ([]core.Summary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+summaryCols+` FROM summaries WHERE session_id = ? ORDER BY at DESC, seq DESC LIMIT ?`,
		id, limitArg(limit))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list summaries: %w", err)
	}
	return collect(rows, scanSummary, "summaries")
}
