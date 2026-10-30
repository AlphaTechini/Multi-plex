package memory

import (
	"cmp"
	"context"
	"slices"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// PutSession inserts or replaces a session by ID.
func (s *Store) PutSession(_ context.Context, sess core.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sess.ID] = cloneSession(sess)
	return nil
}

// GetSession returns the session with the given ID or store.ErrNotFound.
func (s *Store) GetSession(_ context.Context, id core.SessionID) (core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return core.Session{}, store.ErrNotFound
	}
	return sess, nil
}

// ListSessions returns the sessions of a workspace (all when ws is empty),
// ordered by CreatedAt ascending, then ID.
func (s *Store) ListSessions(_ context.Context, ws core.WorkspaceID) ([]core.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]core.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		if ws == "" || sess.WorkspaceID == ws {
			out = append(out, sess)
		}
	}
	slices.SortFunc(out, func(a, b core.Session) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out, nil
}

// DeleteSession removes a session with its summaries, notes, comments and
// checkpoints. The approvals audit log is kept. Deleting a missing session is
// not an error.
func (s *Store) DeleteSession(_ context.Context, id core.SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, id)
	delete(s.summaries, id)
	delete(s.notes, id)
	delete(s.checkpoints, id)
	for cid, c := range s.comments {
		if c.SessionID == id {
			delete(s.comments, cid)
		}
	}
	return nil
}
