package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// dsnQuery configures every connection: WAL, a 5s busy wait, enforced foreign
// keys, relaxed fsync (safe with WAL), and BEGIN IMMEDIATE for transactions so
// writers queue on the busy timeout instead of failing on lock upgrade. The
// driver runs busy_timeout first, then the other pragmas.
const dsnQuery = "?_txlock=immediate" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=journal_mode(WAL)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=foreign_keys(1)"

// Open opens (creating if needed) the database at path, creating missing
// parent directories (0700) and the file itself (0600), and applies any
// pending migrations. The caller must Close the store.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("sqlite: create directory: %w", err)
	}
	// Create the file ourselves so it is private from the start; SQLite gives
	// the -wal and -shm files the same mode.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("sqlite: create database file: %w", err)
	}
	f.Close()

	// The driver cuts the DSN at the first '?', so a plain path (including a
	// Windows path with a drive letter) is passed through untouched.
	db, err := sql.Open("sqlite", path+dsnQuery)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}
	db.SetConnMaxIdleTime(time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}
