package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	_ "embed"
)

//go:embed schema.sql
var schemaSQL string

const schemaVersion = 2

func migrate(ctx context.Context, db *sql.DB) error {
	if ctx == nil {
		ctx = context.Background()
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if _, err := tx.ExecContext(ctx, schemaSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("apply schema: %w", err)
	}

	var versionStr string
	err = tx.QueryRowContext(ctx, "SELECT value FROM metadata WHERE key = 'schema_version'").Scan(&versionStr)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := tx.ExecContext(ctx, "INSERT INTO metadata(key, value) VALUES('schema_version', ?)", strconv.Itoa(schemaVersion)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert schema version: %w", err)
		}
		return tx.Commit()
	}
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("read schema version: %w", err)
	}

	version, err := strconv.Atoi(versionStr)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("parse schema version: %w", err)
	}
	if version > schemaVersion {
		_ = tx.Rollback()
		return fmt.Errorf("database schema version %d is newer than supported %d", version, schemaVersion)
	}
	if version < schemaVersion {
		if _, err := tx.ExecContext(ctx, "UPDATE metadata SET value = ? WHERE key = 'schema_version'", strconv.Itoa(schemaVersion)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update schema version: %w", err)
		}
	}

	return tx.Commit()
}
