package memory

import (
	"context"
	"fmt"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// AddNote stores n with the next per-session Number (1, 2, 3, ...), generating
// an ID when empty, and returns the stored note. A duplicate ID is an error.
func (s *Store) AddNote(_ context.Context, n core.Note) (core.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n.ID == "" {
		n.ID = core.NewID("note")
	}
	for _, list := range s.notes {
		for _, have := range list {
			if have.ID == n.ID {
				return core.Note{}, fmt.Errorf("memory: note %q already exists", n.ID)
			}
		}
	}
	n.Number = len(s.notes[n.SessionID]) + 1
	n = cloneNote(n)
	s.notes[n.SessionID] = append(s.notes[n.SessionID], n)
	return cloneNote(n), nil
}

// ListNotes returns a session's notes ordered by Number ascending.
func (s *Store) ListNotes(_ context.Context, id core.SessionID) ([]core.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	src := s.notes[id]
	out := make([]core.Note, len(src))
	for i, n := range src {
		out[i] = cloneNote(n)
	}
	return out, nil
}

// DismissNote marks a note dismissed. It returns store.ErrNotFound if the
// note does not exist; dismissing twice is not an error.
func (s *Store) DismissNote(_ context.Context, noteID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, list := range s.notes {
		for i := range list {
			if list[i].ID == noteID {
				list[i].Dismissed = true
				return nil
			}
		}
	}
	return store.ErrNotFound
}
