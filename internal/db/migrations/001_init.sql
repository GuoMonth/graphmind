-- Initial schema: nodes, edges, tags, events, proposals

-- Nodes: vertices in the project graph
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL CHECK (type IN ('task','epic','decision','risk','release','discussion')),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT '',
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_nodes_type   ON nodes (type);
CREATE INDEX idx_nodes_status ON nodes (status);

-- Edges: directed relationships between nodes
CREATE TABLE edges (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL CHECK (type IN ('depends_on','blocks','decompose','caused_by','related_to','supersedes')),
    from_id     TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    to_id       TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (type, from_id, to_id),
    CHECK (from_id != to_id)
);

CREATE INDEX idx_edges_type    ON edges (type);
CREATE INDEX idx_edges_from_id ON edges (from_id);
CREATE INDEX idx_edges_to_id   ON edges (to_id);

-- Tags: AI-extracted semantic labels
CREATE TABLE tags (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_tags_name ON tags (name);

-- Node-tag junction table
CREATE TABLE node_tags (
    node_id TEXT NOT NULL REFERENCES nodes (id) ON DELETE CASCADE,
    tag_id  TEXT NOT NULL REFERENCES tags  (id) ON DELETE CASCADE,
    PRIMARY KEY (node_id, tag_id)
);

CREATE INDEX idx_node_tags_tag_id ON node_tags (tag_id);

-- Events: immutable audit log of all mutations
CREATE TABLE events (
    id          TEXT PRIMARY KEY,
    entity_type TEXT NOT NULL,
    entity_id   TEXT NOT NULL,
    action      TEXT NOT NULL,
    payload     TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_events_entity    ON events (entity_type, entity_id);
CREATE INDEX idx_events_action    ON events (action);
CREATE INDEX idx_events_created   ON events (created_at);

-- Proposals: staged batches of changes
CREATE TABLE proposals (
    id          TEXT PRIMARY KEY,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','committed','rejected')),
    operations  TEXT NOT NULL DEFAULT '[]',
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_proposals_status ON proposals (status);

-- FTS5 virtual tables for full-text search
CREATE VIRTUAL TABLE nodes_fts USING fts5 (
    title, description,
    content='nodes',
    content_rowid='rowid'
);

-- Triggers to keep FTS5 in sync with nodes
CREATE TRIGGER nodes_fts_insert AFTER INSERT ON nodes BEGIN
    INSERT INTO nodes_fts (rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

CREATE TRIGGER nodes_fts_delete AFTER DELETE ON nodes BEGIN
    INSERT INTO nodes_fts (nodes_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
END;

CREATE TRIGGER nodes_fts_update AFTER UPDATE ON nodes BEGIN
    INSERT INTO nodes_fts (nodes_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
    INSERT INTO nodes_fts (nodes_fts, rowid, title, description) VALUES (new.rowid, new.title, new.description);
END;

CREATE VIRTUAL TABLE tags_fts USING fts5 (
    name, description,
    content='tags',
    content_rowid='rowid'
);

CREATE TRIGGER tags_fts_insert AFTER INSERT ON tags BEGIN
    INSERT INTO tags_fts (rowid, name, description) VALUES (new.rowid, new.name, new.description);
END;

CREATE TRIGGER tags_fts_delete AFTER DELETE ON tags BEGIN
    INSERT INTO tags_fts (tags_fts, rowid, name, description) VALUES ('delete', old.rowid, old.name, old.description);
END;

CREATE TRIGGER tags_fts_update AFTER UPDATE ON tags BEGIN
    INSERT INTO tags_fts (tags_fts, rowid, name, description) VALUES ('delete', old.rowid, old.name, old.description);
    INSERT INTO tags_fts (tags_fts, rowid, name, description) VALUES (new.rowid, new.name, new.description);
END;
