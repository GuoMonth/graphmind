-- Migration 002: Open type system + tag-to-tag edges + event fields
--
-- Changes:
--   1. Remove CHECK constraints on nodes.type and edges.type (open type system)
--   2. Add who, where, event_time columns to nodes
--   3. Create tag_edges table for tag-to-tag relationships

-- SQLite does not support ALTER TABLE DROP CONSTRAINT.
-- We must recreate the tables to remove CHECK constraints.

-- Step 1: Recreate nodes table without type CHECK, add new columns
CREATE TABLE nodes_new (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    who         TEXT NOT NULL DEFAULT '',
    "where"     TEXT NOT NULL DEFAULT '',
    event_time  TEXT NOT NULL DEFAULT '',
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

INSERT INTO nodes_new (id, type, title, description, status, properties, created_at, updated_at)
    SELECT id, type, title, description, status, properties, created_at, updated_at FROM nodes;

-- Drop FTS triggers before dropping nodes
DROP TRIGGER IF EXISTS nodes_fts_insert;
DROP TRIGGER IF EXISTS nodes_fts_delete;
DROP TRIGGER IF EXISTS nodes_fts_update;

-- Drop old indexes
DROP INDEX IF EXISTS idx_nodes_type;
DROP INDEX IF EXISTS idx_nodes_status;

DROP TABLE nodes;
ALTER TABLE nodes_new RENAME TO nodes;

CREATE INDEX idx_nodes_type   ON nodes (type);
CREATE INDEX idx_nodes_status ON nodes (status);

-- Recreate FTS triggers
CREATE TRIGGER nodes_fts_insert AFTER INSERT ON nodes BEGIN
    INSERT INTO nodes_fts (rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

CREATE TRIGGER nodes_fts_delete AFTER DELETE ON nodes BEGIN
    INSERT INTO nodes_fts (nodes_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
END;

CREATE TRIGGER nodes_fts_update AFTER UPDATE ON nodes BEGIN
    INSERT INTO nodes_fts (nodes_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
    INSERT INTO nodes_fts (rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

-- Step 2: Recreate edges table without type CHECK
CREATE TABLE edges_new (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    from_id     TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    to_id       TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (type, from_id, to_id),
    CHECK (from_id != to_id)
);

INSERT INTO edges_new (id, type, from_id, to_id, properties, created_at, updated_at)
    SELECT id, type, from_id, to_id, properties, created_at, updated_at FROM edges;

DROP INDEX IF EXISTS idx_edges_type;
DROP INDEX IF EXISTS idx_edges_from_id;
DROP INDEX IF EXISTS idx_edges_to_id;

DROP TABLE edges;
ALTER TABLE edges_new RENAME TO edges;

CREATE INDEX idx_edges_type    ON edges (type);
CREATE INDEX idx_edges_from_id ON edges (from_id);
CREATE INDEX idx_edges_to_id   ON edges (to_id);

-- Step 3: Create tag_edges table
CREATE TABLE tag_edges (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    from_id     TEXT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    to_id       TEXT NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (type, from_id, to_id),
    CHECK (from_id != to_id)
);

CREATE INDEX idx_tag_edges_type    ON tag_edges (type);
CREATE INDEX idx_tag_edges_from_id ON tag_edges (from_id);
CREATE INDEX idx_tag_edges_to_id   ON tag_edges (to_id);
