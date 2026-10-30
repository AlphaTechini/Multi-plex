// Package sqlite is the SQLite (modernc.org/sqlite, pure Go) implementation
// of store.Store. See docs/01-architecture.md "Storage".
//
// Encoding: times are unix nanoseconds (0 is the zero time) and come back in
// UTC; string slices are JSON text; booleans are 0/1.
package sqlite

import (
	"database/sql"

	"github.com/alphatechini/multi-plex/internal/store"
)

// Store is a store.Store backed by one SQLite database file.
type Store struct {
	db *sql.DB
}

var _ store.Store = (*Store)(nil)

// Close checkpoints the write-ahead log (best effort) and closes the database.
func (s *Store) Close() error {
	_, _ = s.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return s.db.Close()
}
