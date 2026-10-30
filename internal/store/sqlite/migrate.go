package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
)

//go:embed schema.sql
var schemaV1 string

// migrations are applied in order; migration i brings the database to version
// i+1. Only ever append to this slice, never edit an entry that has shipped.
var migrations = []string{
	schemaV1,
}

// migrate applies every migration newer than the version recorded in the
// schema_version table. Each runs in its own transaction, and the version is
// re-read inside it, so two processes opening a fresh database cannot both
// apply the same migration.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("sqlite: create schema_version: %w", err)
	}
	for i, m := range migrations {
		if err := applyMigration(ctx, db, i+1, m); err != nil {
			return fmt.Errorf("sqlite: migration %d: %w", i+1, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, version int, script string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var current int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&current); err != nil {
		return err
	}
	if current >= version {
		return nil
	}
	if _, err := tx.ExecContext(ctx, script); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_version`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, version); err != nil {
		return err
	}
	return tx.Commit()
}
