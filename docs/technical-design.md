# Technical Design

> For system-level architecture (layers, data flow, interfaces, lifecycle), see [Architecture](architecture.md).
> This document covers storage decisions, data model schemas, and SQL-level patterns.

## Storage: SQLite + Adjacency List

GraphMind uses a single SQLite database file to store the full project graph. No external database server is needed.

### Why SQLite

- **Local-first, zero config** — single `.db` file, no server, no network
- **OLTP fit** — node/edge CRUD and event appending are transactional workloads, exactly what SQLite excels at
- **Graph traversal** — `WITH RECURSIVE` CTEs handle dependency chains, impact analysis, and subgraph extraction at project-management scale (hundreds to low thousands of nodes)
- **Flexible properties** — JSON columns + `json_extract()` for schemaless node/edge attributes
- **Go ecosystem** — mature drivers (`modernc.org/sqlite` or `mattn/go-sqlite3`)

### Why not other databases

| Alternative | Rejected because |
|---|---|
| Neo4j | Requires standalone server. Violates local-first, zero-config principle |
| DGraph / Cayley | Adds deployment complexity. Not lightweight enough for embedding |
| PostgreSQL | Not suitable for single-machine zero-config use |
| DuckDB | OLAP-oriented. Optimized for bulk analytics, not single-row event appending. Unnecessary at project-management scale |
| Plain JSON files | No query capability, no transactions, no concurrent safety |

### When to reconsider

DuckDB could be introduced later as a **read-only analytics layer** (it can read SQLite files directly) if cross-project large-scale analysis becomes a requirement. This is not a current-phase concern.

---

## Data Model

### Event Store (source of truth)

All mutations are recorded as immutable events. The current graph state is derived by replaying or projecting these events.

```sql
CREATE TABLE events (
    id          TEXT PRIMARY KEY,  -- UUID v7
    type        TEXT NOT NULL,     -- e.g. node_created, edge_created, node_updated,
                                   --      proposal_committed, node_deleted
    payload     TEXT NOT NULL,     -- JSON
    created_at  TEXT NOT NULL      -- ISO 8601
);

CREATE INDEX idx_events_type ON events(type);
CREATE INDEX idx_events_created_at ON events(created_at);
```

### Nodes (projection)

Materialized current state of all graph nodes.

```sql
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,  -- UUID v7
    type        TEXT NOT NULL,     -- e.g. task, epic, decision, risk, release
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open',
    properties  TEXT DEFAULT '{}', -- JSON for flexible attributes
    created_at  TEXT NOT NULL,     -- ISO 8601
    updated_at  TEXT NOT NULL      -- ISO 8601
);

CREATE INDEX idx_nodes_type ON nodes(type);
CREATE INDEX idx_nodes_status ON nodes(status);
```

### Edges (projection)

Materialized current state of all graph relationships.

```sql
CREATE TABLE edges (
    id          TEXT PRIMARY KEY,  -- UUID v7
    type        TEXT NOT NULL,     -- e.g. depends_on, blocks, decompose, caused_by
    from_id     TEXT NOT NULL REFERENCES nodes(id),
    to_id       TEXT NOT NULL REFERENCES nodes(id),
    properties  TEXT DEFAULT '{}', -- JSON
    created_at  TEXT NOT NULL,     -- ISO 8601
    updated_at  TEXT NOT NULL      -- ISO 8601
);

CREATE INDEX idx_edges_from ON edges(from_id);
CREATE INDEX idx_edges_to ON edges(to_id);
CREATE INDEX idx_edges_type ON edges(type);
```

### Proposals (staging area)

All changes are staged as proposals before being committed to the graph.

```sql
CREATE TABLE proposals (
    id            TEXT PRIMARY KEY,  -- UUID v7
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending, committed, rejected
    payload       TEXT NOT NULL,     -- JSON: proposed nodes/edges/updates
    created_at    TEXT NOT NULL,     -- ISO 8601
    committed_at  TEXT               -- ISO 8601, NULL until committed
);

CREATE INDEX idx_proposals_status ON proposals(status);
```

### Tags (AI-extracted semantic anchors)

Tags are named concepts that AI extracts from node content. Nodes sharing tags are implicitly related.

```sql
CREATE TABLE tags (
    id          TEXT PRIMARY KEY,   -- UUID v7
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL,      -- ISO 8601
    updated_at  TEXT NOT NULL       -- ISO 8601
);

CREATE TABLE node_tags (
    node_id  TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    tag_id   TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (node_id, tag_id)
);

CREATE INDEX idx_node_tags_tag_id ON node_tags(tag_id);
```

### Full-Text Search (FTS5)

SQLite FTS5 virtual tables for keyword search across nodes and tags:

```sql
CREATE VIRTUAL TABLE nodes_fts USING fts5(title, properties, content='nodes', content_rowid='rowid');
CREATE VIRTUAL TABLE tags_fts USING fts5(name, description, content='tags', content_rowid='rowid');
```

FTS5 tables are kept in sync with projection tables on every write.

---

## Key Design Decisions

### UUID v7 as primary keys

All tables use UUID v7 as primary keys. UUID v7 is time-ordered, which means:

- Natural chronological ordering without extra columns
- Efficient B-tree index insertion (always appending near the end)
- Globally unique across tables and potential future distributed scenarios

### Event sourcing

The `events` table is the **single source of truth**. The `nodes` and `edges` tables are **projections** (materialized views) that can be rebuilt by replaying events from scratch.

Benefits:

- Full audit trail of every change
- Ability to reconstruct graph state at any point in time
- Evolution analysis (how the project understanding changed over time)

### JSON properties

Nodes and edges have a `properties` JSON column for flexible, schemaless attributes (assignee, priority, deadline, tags, etc.). This avoids schema migrations when new attributes are needed.

Queryable via SQLite's JSON functions:

```sql
SELECT * FROM nodes
WHERE json_extract(properties, '$.assignee') = 'Alice';
```

### Graph traversal via recursive CTEs

Example — find all transitive dependencies of a node:

```sql
WITH RECURSIVE deps(id) AS (
    SELECT to_id FROM edges WHERE from_id = :node_id AND type = 'depends_on'
    UNION
    SELECT e.to_id FROM edges e JOIN deps d ON e.from_id = d.id
    WHERE e.type = 'depends_on'
)
SELECT n.* FROM nodes n JOIN deps d ON n.id = d.id;
```

### Cycle detection

Graphs may contain cycles (A depends on B, B depends on A). Recursive CTEs must explicitly track visited nodes to prevent infinite recursion:

```sql
WITH RECURSIVE deps(id, path) AS (
    SELECT to_id, ',' || :node_id || ',' || to_id || ','
    FROM edges WHERE from_id = :node_id AND type = 'depends_on'
    UNION
    SELECT e.to_id, d.path || e.to_id || ','
    FROM edges e JOIN deps d ON e.from_id = d.id
    WHERE e.type = 'depends_on'
      AND d.path NOT LIKE '%,' || e.to_id || ',%'
)
SELECT DISTINCT n.* FROM nodes n JOIN deps d ON n.id = d.id;
```

---

## CLI Design

See [CLI Specification](cli-spec.md) for the complete command reference and AI-friendly design principles.
