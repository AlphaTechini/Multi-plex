// Package memory is an in-memory store.Store for tests and for running the
// daemon without persistence. It copies everything that goes in and out, so
// callers can never mutate stored data through a shared slice.
package memory

import (
	"sync"

	"github.com/alphatechini/multi-plex/internal/core"
	"github.com/alphatechini/multi-plex/internal/store"
)

// Store is a mutex-protected in-memory implementation of store.Store.
type Store struct {
	mu          sync.Mutex
	workspaces  map[core.WorkspaceID]core.Workspace
	sessions    map[core.SessionID]core.Session
	summaries   map[core.SessionID][]core.Summary // insertion order
	notes       map[core.SessionID][]core.Note    // insertion order, Number ascending
	comments    map[string]core.Comment
	checkpoints map[core.SessionID][]core.Checkpoint
	approvals   []store.AuditEntry // insertion order, never modified
}

var _ store.Store = (*Store)(nil)

// New returns an empty store.
func New() *Store {
	return &Store{
		workspaces:  map[core.WorkspaceID]core.Workspace{},
		sessions:    map[core.SessionID]core.Session{},
		summaries:   map[core.SessionID][]core.Summary{},
		notes:       map[core.SessionID][]core.Note{},
		comments:    map[string]core.Comment{},
		checkpoints: map[core.SessionID][]core.Checkpoint{},
	}
}

// Close is a no-op; the data stays readable.
func (s *Store) Close() error { return nil }
