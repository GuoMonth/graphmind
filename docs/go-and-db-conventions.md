# Go & Database Conventions

Hard rules and conventions for the GraphMind codebase. All contributors (human and AI) must follow these.

---

## Go Version

- **Go 1.26** (minimum and target)
- `go.mod` must specify `go 1.26`
- Use Go 1.26 features where appropriate:
  - `new(expr)` for pointer initialization (e.g., `new("default")` instead of `ptr := "default"; &ptr`)
  - Self-referential generic constraints where they simplify code
  - `go fix` for codebase modernization

---

## Project Structure

```
graphmind/
├── cmd/
│   └── gm/
│       └── main.go              # CLI entrypoint
├── internal/
│   ├── db/
│   │   ├── db.go                # Connection management, PRAGMA setup
│   │   ├── migrate.go           # Schema migrations (embedded SQL)
│   │   └── migrations/          # .sql migration files
│   │       ├── 001_init.sql
│   │       └── ...
│   ├── model/
│   │   ├── node.go              # Node struct + methods
│   │   ├── edge.go              # Edge struct + methods
│   │   ├── event.go             # Event struct + methods
│   │   └── proposal.go          # Proposal struct + methods
│   ├── graph/
│   │   ├── store.go             # Graph read/write operations
│   │   ├── traverse.go          # Traversal queries
│   │   └── validate.go          # Cycle detection, consistency checks
│   ├── proposal/
│   │   └── service.go           # Proposal create/commit/reject logic
│   ├── event/
│   │   └── store.go             # Event append + query
│   └── cli/
│       ├── root.go              # Root command setup
│       ├── node.go              # gm node <action>
│       ├── edge.go              # gm edge <action>
│       ├── proposal.go          # gm proposal <action>
│       ├── graph.go             # gm graph <action>
│       ├── event.go             # gm event list
│       ├── schema.go            # gm schema
│       └── output.go            # JSON envelope helpers
├── docs/
├── go.mod
├── go.sum
├── README.md
└── LICENSE
```

### Rules

- `cmd/` — only entrypoints. No business logic.
- `internal/` — all business logic. Not importable by external packages.
- `internal/cli/` — command handlers. Parse flags, call services, format output.
- `internal/model/` — domain types. No database imports.
- `internal/db/` — database connection and migrations only.
- `internal/graph/`, `internal/proposal/`, `internal/event/` — service layer. Depends on `model/` and `db/`.

---

## Dependencies

### Core dependencies

| Package | Purpose | Version |
|---|---|---|
| `modernc.org/sqlite` | SQLite driver (pure Go, no CGO) | latest |
| `github.com/google/uuid` | UUID v7 generation | latest |
| `github.com/spf13/cobra` | CLI framework | latest |

### Rules

- **Minimize dependencies.** Every new dependency must justify its existence.
- **No ORM.** Use `database/sql` directly. SQL is the interface to SQLite; wrapping it adds indirection without value.
- **No CGO.** The binary must be a single static executable with zero system dependencies.

---

## UUID v7

All primary keys across all tables use UUID v7 (RFC 9562).

### Generation

```go
import "github.com/google/uuid"

func NewID() string {
    return uuid.Must(uuid.NewV7()).String()
}
```

### Rules

- Primary keys are always `TEXT` in SQLite (storing the UUID string representation).
- Always generate UUID v7 in Go code, never in SQL.
- UUID v7 is time-ordered: the first 48 bits encode millisecond-precision Unix timestamp. This gives natural chronological ordering in B-tree indexes.
- When Go stdlib `crypto/uuid` lands (expected Go 1.27), migrate to it.

---

## SQLite Conventions

### Connection setup

Every database connection must apply these PRAGMAs at open:

```sql
PRAGMA journal_mode = WAL;          -- Write-Ahead Logging for concurrent reads
PRAGMA foreign_keys = ON;            -- Enforce foreign key constraints
PRAGMA busy_timeout = 5000;          -- Wait 5s on lock contention instead of failing immediately
PRAGMA synchronous = NORMAL;         -- Safe with WAL mode, better write performance
PRAGMA cache_size = -64000;          -- 64MB page cache
PRAGMA temp_store = MEMORY;          -- Keep temp tables in memory
```

### Data types

| Go type | SQLite type | Notes |
|---|---|---|
| UUID string | `TEXT` | Always stored as lowercase hyphenated string |
| Timestamps | `TEXT` | ISO 8601 format (`2026-04-02T07:00:00Z`), always UTC |
| JSON data | `TEXT` | Validated in Go before writing. Queryable via `json_extract()` |
| Enums (status, type) | `TEXT` | Validated in Go. Use CHECK constraints as safety net |
| Booleans | `INTEGER` | 0 or 1 |

### Naming

- Table names: **plural**, **snake_case** (`nodes`, `edges`, `events`, `proposals`)
- Column names: **singular**, **snake_case** (`created_at`, `from_id`, `node_type`)
- Index names: `idx_{table}_{column}` (e.g., `idx_nodes_type`, `idx_edges_from_id`)
- Foreign keys always end with `_id`

### Transactions

- All write operations (create, update, delete) must run inside an explicit transaction.
- Proposal commit is **atomic**: all operations succeed or none do.
- Read operations do not require explicit transactions (WAL mode gives consistent reads).

### JSON properties

Nodes and edges have a `properties TEXT` column for flexible attributes.

```go
// Writing
props, _ := json.Marshal(map[string]any{"assignee": "Alice", "priority": "high"})
// ... INSERT INTO nodes (..., properties) VALUES (..., ?)

// Querying in SQL
// SELECT * FROM nodes WHERE json_extract(properties, '$.assignee') = 'Alice'
```

Rules:
- Always validate JSON in Go before writing.
- `properties` defaults to `'{}'`, never `NULL`.
- Use `json_extract()` for SQL-level filtering.

---

## Schema Migrations

### Strategy

- **Embedded SQL files** via Go `embed` package.
- Numbered sequentially: `001_init.sql`, `002_add_tags.sql`, ...
- Each file contains both the DDL and a version record.
- Migrations are **forward-only**. No rollback files. If a migration is wrong, write a new migration to fix it.
- Applied automatically at startup.

### Migration tracking

```sql
CREATE TABLE schema_version (
    version  INTEGER PRIMARY KEY,
    name     TEXT NOT NULL,
    applied  TEXT NOT NULL          -- ISO 8601 timestamp
);
```

### Initial migration (001_init.sql)

```sql
-- Events (source of truth)
CREATE TABLE events (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL,
    payload     TEXT NOT NULL,
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_events_type ON events(type);
CREATE INDEX idx_events_created_at ON events(created_at);

-- Nodes (projection)
CREATE TABLE nodes (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL CHECK(type IN ('task','epic','decision','risk','release','discussion')),
    title       TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'open',
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX idx_nodes_type ON nodes(type);
CREATE INDEX idx_nodes_status ON nodes(status);

-- Edges (projection)
CREATE TABLE edges (
    id          TEXT PRIMARY KEY,
    type        TEXT NOT NULL CHECK(type IN ('depends_on','blocks','decompose','caused_by','related_to','supersedes')),
    from_id     TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    to_id       TEXT NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
    properties  TEXT NOT NULL DEFAULT '{}',
    created_at  TEXT NOT NULL,
    updated_at  TEXT NOT NULL
);
CREATE INDEX idx_edges_from_id ON edges(from_id);
CREATE INDEX idx_edges_to_id ON edges(to_id);
CREATE INDEX idx_edges_type ON edges(type);

-- Proposals (staging)
CREATE TABLE proposals (
    id            TEXT PRIMARY KEY,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','committed','rejected')),
    description   TEXT NOT NULL DEFAULT '',
    payload       TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    committed_at  TEXT
);
CREATE INDEX idx_proposals_status ON proposals(status);

-- Schema version tracking
CREATE TABLE schema_version (
    version  INTEGER PRIMARY KEY,
    name     TEXT NOT NULL,
    applied  TEXT NOT NULL
);
INSERT INTO schema_version (version, name, applied)
VALUES (1, '001_init', strftime('%Y-%m-%dT%H:%M:%SZ', 'now'));
```

---

## Error Handling

- Use Go's standard `error` interface. No custom error frameworks.
- Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the error chain.
- Define sentinel errors for domain-level conditions:

```go
var (
    ErrNotFound     = errors.New("not found")
    ErrConflict     = errors.New("conflict")
    ErrInvalidInput = errors.New("invalid input")
    ErrInvalidState = errors.New("invalid state")
)
```

- CLI layer maps domain errors to exit codes and JSON error responses.

---

## Testing

- Use Go's standard `testing` package. No third-party test frameworks.
- Use `testing/fstest`, table-driven tests, and subtests.
- Database tests use an **in-memory SQLite** (`:memory:`) — fast, isolated, no cleanup needed.
- Test file naming: `*_test.go` in the same package.

```go
func TestNodeCreate(t *testing.T) {
    db := testDB(t)     // opens :memory:, runs migrations
    store := graph.NewStore(db)

    node, err := store.CreateNode(ctx, model.CreateNodeInput{
        Type:  "task",
        Title: "Test task",
    })
    if err != nil {
        t.Fatalf("CreateNode: %v", err)
    }
    if node.Title != "Test task" {
        t.Errorf("got title %q, want %q", node.Title, "Test task")
    }
}
```

---

## Code Style

- Follow `gofmt` and `go vet`. No exceptions.
- Run `go fix` for modernization.
- No globals except sentinel errors.
- Context (`context.Context`) as first parameter for all service-layer functions.
- Return `(result, error)`, never `panic` for business logic.
- JSON struct tags use `snake_case` to match CLI output and database columns:

```go
type Node struct {
    ID         string         `json:"id"`
    Type       string         `json:"type"`
    Title      string         `json:"title"`
    Status     string         `json:"status"`
    Properties map[string]any `json:"properties"`
    CreatedAt  string         `json:"created_at"`
    UpdatedAt  string         `json:"updated_at"`
}
```
