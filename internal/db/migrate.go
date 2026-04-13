package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Migrate applies all pending migrations in order.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := ensureSchemaVersion(ctx, db); err != nil {
		return fmt.Errorf("ensure schema_version table: %w", err)
	}

	current, err := currentVersion(ctx, db)
	if err != nil {
		return fmt.Errorf("get current version: %w", err)
	}

	files, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migration files: %w", err)
	}

	var names []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".sql") {
			names = append(names, f.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if version <= current {
			continue
		}

		content, err := migrationFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("execute migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO schema_version (version, name) VALUES (?, ?)",
			version, name,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

func ensureSchemaVersion(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_version (
			version TEXT PRIMARY KEY,
			name    TEXT NOT NULL,
			applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
		)
	`)
	return err
}

func currentVersion(ctx context.Context, db *sql.DB) (string, error) {
	var version sql.NullString
	err := db.QueryRowContext(ctx,
		"SELECT MAX(version) FROM schema_version",
	).Scan(&version)
	if err != nil {
		return "", err
	}
	if !version.Valid {
		return "", nil
	}
	return version.String, nil
}
