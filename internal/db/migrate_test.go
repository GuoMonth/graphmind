package db_test

import (
	"context"
	"testing"

	"github.com/senguoyun-guosheng/graphmind/internal/db"
)

func TestOpenMemory(t *testing.T) {
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer d.Close()

	// Verify PRAGMAs
	tests := []struct {
		pragma string
		want   string
	}{
		{"PRAGMA foreign_keys", "1"},
		{"PRAGMA busy_timeout", "5000"},
		{"PRAGMA synchronous", "1"}, // NORMAL = 1
		{"PRAGMA temp_store", "2"},  // MEMORY = 2
	}
	for _, tt := range tests {
		var got string
		if err := d.QueryRow(tt.pragma).Scan(&got); err != nil {
			t.Errorf("%s: %v", tt.pragma, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s = %q, want %q", tt.pragma, got, tt.want)
		}
	}
}

func TestMigrate(t *testing.T) {
	ctx := context.Background()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer d.Close()

	// First migration
	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate (first): %v", err)
	}

	// Verify core tables exist
	tables := []string{"nodes", "edges", "tags", "node_tags", "tag_edges", "events", "proposals", "schema_version"}
	for _, tbl := range tables {
		var name string
		err := d.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", tbl,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after migration: %v", tbl, err)
		}
	}

	// Idempotent: run again should not fail
	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate (idempotent): %v", err)
	}

	// Verify schema_version has entries for all migrations
	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil {
		t.Fatalf("count schema_version: %v", err)
	}
	if count != 2 {
		t.Errorf("schema_version rows = %d, want 2", count)
	}
}

func TestMigrateForeignKeys(t *testing.T) {
	ctx := context.Background()
	d, err := db.OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer d.Close()

	if err := db.Migrate(ctx, d); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Inserting an edge referencing a non-existent node should fail
	_, err = d.ExecContext(ctx,
		`INSERT INTO edges (id, type, from_id, to_id) VALUES ('e1', 'depends_on', 'nonexistent', 'also_nonexistent')`,
	)
	if err == nil {
		t.Error("expected foreign key violation, got nil")
	}
}
