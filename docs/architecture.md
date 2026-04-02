# Architecture

## Vision Recap

GraphMind is an AI-agent-native, local-first project management tool. The core thesis:

1. **Real projects are graphs** — not flat lists, not Kanban boards
2. **AI agents operate the tool** — humans talk to Claude Code / Codex / Copilot, which call the `gm` CLI
3. **Events are truth** — all mutations are recorded as immutable events; current state is a projection
4. **Proposals gate all writes** — AI agents stage changes, humans confirm, then commit
5. **Single binary, single file** — Go 1.26, pure Go (no CGO), SQLite in WAL mode

---

## System Layers

```
┌─────────────────────────────────────────────────────────────────┐
│                        CLI Layer (cobra)                        │
│  Parse flags/stdin → dispatch to services → JSON envelope out   │
├─────────────────────────────────────────────────────────────────┤
│                       Service Layer                             │
│  ┌──────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │ GraphService  │  │ ProposalService  │  │  EventService    │  │
│  │              │  │                  │  │                  │  │
│  │ CRUD nodes   │  │ Create proposal  │  │ Append events    │  │
│  │ CRUD edges   │  │ Validate ops     │  │ Query event log  │  │
│  │ Traverse     │  │ Commit (atomic)  │  │                  │  │
│  │ Query/search │  │ Reject           │  │                  │  │
│  └──────┬───────┘  └────────┬─────────┘  └────────┬─────────┘  │
│         │                   │                      │            │
├─────────┴───────────────────┴──────────────────────┴────────────┤
│                       Domain Model                              │
│  Node, Edge, Tag, Event, Proposal — pure Go structs, no DB imports  │
├─────────────────────────────────────────────────────────────────┤
│                      Database Layer                              │
│  Connection management, PRAGMAs, migrations, sql.DB pool        │
├─────────────────────────────────────────────────────────────────┤
│                     SQLite (single file)                         │
│  WAL mode · foreign keys · JSON functions · recursive CTEs      │
└─────────────────────────────────────────────────────────────────┘
```

### Layer rules

| Layer | Can depend on | Cannot depend on |
|---|---|---|
| CLI | Service, Model | Database directly |
| Service | Model, Database | CLI |
| Model | Nothing (stdlib only) | Any internal package |
| Database | Nothing (stdlib + driver) | Any internal package |

---

## Package Architecture

```
cmd/gm/main.go
  │
  └─→ internal/cli          ← cobra commands, JSON I/O
        │
        ├─→ internal/graph       ← GraphService (node/edge CRUD, traversal, validation)
        ├─→ internal/tag         ← TagService (tag CRUD, FTS5 search)
        ├─→ internal/proposal    ← ProposalService (create, validate, commit, reject)
        └─→ internal/event       ← EventService (append, query)
              │
              └─→ internal/model     ← pure domain types (Node, Edge, Tag, Event, Proposal)
                    (no deps)

        All services also depend on:
              └─→ internal/db        ← Open(), Migrate(), connection pool
                    (no internal deps)
```

### Dependency flow (strict, enforced by `internal/`)

```
cli → graph, tag, proposal, event
graph → model, db, event
tag → model, db, event
proposal → model, db, event, graph, tag
event → model, db
model → (nothing)
db → (nothing)
```

No package may import a package that imports it (no cycles).

---

## Data Flow

### Read Path (query / list / traverse)

```
AI Agent
  │  $ gm node list --type task --status open
  ▼
CLI Layer
  │  parse flags
  ▼
GraphService.ListNodes(ctx, filter)
  │  SELECT * FROM nodes WHERE type = ? AND status = ?
  ▼
SQLite (nodes table — projection)
  │
  ▼
[]model.Node
  │
  ▼
CLI Layer
  │  wrap in {ok: true, data: [...]}
  ▼
stdout (JSON)
```

Reads go directly against projection tables. No event replay. Fast and simple.

### Write Path (direct node/edge CRUD)

```
AI Agent
  │  echo '{"type":"task","title":"Design API"}' | gm node create
  ▼
CLI Layer
  │  parse stdin JSON → model.CreateNodeInput
  ▼
GraphService.CreateNode(ctx, input)
  │
  │  BEGIN TX
  │    1. Validate input
  │    2. Generate UUID v7
  │    3. INSERT INTO nodes (...)
  │    4. EventService.Append(event{type: "node_created", payload: ...})
  │  COMMIT TX
  │
  ▼
model.Node (created)
  │
  ▼
CLI Layer → stdout (JSON)
```

Every write:
1. Validates input
2. Mutates the projection table
3. Appends an event (in the same transaction)

Projection and event store are **always in sync** — guaranteed by a single SQLite transaction.

### Proposal Flow (the primary write path for AI agents)

```
AI Agent
  │  cat proposal.json | gm proposal create
  ▼
CLI Layer
  │  parse stdin JSON → model.CreateProposalInput
  ▼
ProposalService.Create(ctx, input)
  │
  │  1. Validate all operations (types, references, no cycles)
  │  2. Generate UUID v7 for proposal
  │  3. INSERT INTO proposals (status='pending', payload=...)
  │
  ▼
model.Proposal (pending)
  │
  ▼
stdout (JSON) → AI Agent presents to human → human confirms
  │
  │  $ gm proposal commit --id <proposal-id>
  ▼
ProposalService.Commit(ctx, proposalID)
  │
  │  BEGIN TX
  │    1. Load proposal, verify status == 'pending'
  │    2. Re-validate all operations against current graph state
  │    3. For each operation:
  │       a. Apply to projection (INSERT/UPDATE/DELETE nodes/edges)
  │       b. Append event
  │    4. UPDATE proposals SET status='committed', committed_at=now()
  │  COMMIT TX
  │
  ▼
model.Proposal (committed) + list of created/updated entities
```

Key properties:
- **Atomic** — all operations in a proposal succeed or none do (single transaction)
- **Re-validated at commit time** — the graph may have changed since the proposal was created
- **Events are appended per-operation** — full granularity in the event log

---

## Event Sourcing Mechanics

### Event Types

| Event type | Payload contains |
|---|---|
| `node_created` | Full node snapshot |
| `node_updated` | Node ID + changed fields (before/after) |
| `node_deleted` | Node ID + last snapshot |
| `edge_created` | Full edge snapshot |
| `edge_deleted` | Edge ID + last snapshot |
| `tag_created` | Full tag snapshot |
| `tag_updated` | Tag ID + changed fields |
| `tag_deleted` | Tag ID + last snapshot |
| `node_tagged` | Node ID + Tag ID |
| `node_untagged` | Node ID + Tag ID |
| `proposal_created` | Proposal ID + description |
| `proposal_committed` | Proposal ID + list of operation event IDs |
| `proposal_rejected` | Proposal ID + reason (optional) |

### Projection Strategy

GraphMind uses **inline projection** (not async):

```
Write operation
  → INSERT event INTO events
  → UPDATE projection table (nodes / edges / proposals)
  → Both in the same transaction
```

Why inline, not async?
- **Single process, single SQLite file** — no need for message queues or async consumers
- **Guaranteed consistency** — projection can never lag behind events
- **Simple** — no background workers, no retry logic, no eventual consistency

### Rebuild from events

If projections are ever corrupted, they can be rebuilt:

```
1. DROP TABLE nodes; DROP TABLE edges; DROP TABLE proposals;
2. Re-create schema (empty tables)
3. SELECT * FROM events ORDER BY created_at
4. For each event, apply to projection
```

This is a **disaster recovery** path, not a normal operation. The `gm` CLI may expose this as `gm graph rebuild` in the future.

---

## Search & Association Model

GraphMind uses a three-layer association model — from coarse to precise:

```
Layer 1: Tags (low cost, medium signal)
  AI extracts tags → nodes share tags → implicit clustering

Layer 2: Explicit Edges (high cost, strong signal)
  AI infers typed relationships → depends_on, blocks, decompose, ...

Layer 3: AI Semantic Understanding (zero cost, broad but weak)
  AI reads node content at query time → deeper reasoning
```

| Layer | Cost to build | Signal strength | Who builds it | Used for |
|---|---|---|---|---|
| **Tags** | Low (AI auto-extracts) | Medium (thematic clustering) | AI, auto | Search entry point, grouping, discovery |
| **Edges** | High (infer type + direction) | Strong (precise structural relationship) | AI + human confirm | Dependency analysis, task decomposition |
| **Semantic** | None (content itself) | Broad but weak | AI at query time | Deep association discovery |

### Tags

Tags are **AI-extracted semantic anchors** — named concepts that recur across the project.

- Each tag has a `name` and a `description` (both AI-generated, evolving over time)
- Nodes can have multiple tags (many-to-many via `node_tags`)
- Two nodes sharing a tag are **implicitly related** without an explicit edge
- Tags create O(N) relationships vs O(N²) explicit edges — scalable

Tag lifecycle:
1. AI extracts candidate tags from user input
2. AI checks existing tags via `gm tag search` — reuse before creating new
3. New tags go through proposal mechanism
4. AI periodically merges synonymous tags (e.g., "payment" + "payments" → "payment")

### Full-Text Search (FTS5)

SQLite FTS5 powers keyword search across nodes and tags:

```sql
-- Node search (title + properties)
CREATE VIRTUAL TABLE nodes_fts USING fts5(title, properties, content='nodes', content_rowid='rowid');

-- Tag search (name + description)
CREATE VIRTUAL TABLE tags_fts USING fts5(name, description, content='tags', content_rowid='rowid');
```

FTS5 is kept in sync with projection tables via triggers or application-level writes.

### Search Flow (how AI agents find context)

```
AI Agent receives new input from human
  │
  ├─ Step 1: gm tag search --keyword "payment"
  │    → Find relevant tags
  │
  ├─ Step 2: gm graph query --tag-name "payment-module" --expand 2
  │    → Find all nodes with this tag + 2-hop neighborhood
  │
  ├─ Step 3: gm graph query --keyword "migration deadline"
  │    → FTS5 keyword search for more specific matches
  │
  ├─ Step 4: (AI reasoning) Read node content, understand relationships
  │    → Semantic layer — no CLI call, happens in the AI agent
  │
  └─ AI Agent now has full context to create/update proposals
```

The `--expand N` flag on `gm graph query` is critical — it lets the AI agent retrieve an anchor node **plus its neighborhood** in a single call, reducing round-trips.

### Event Types for Tags

| Event type | Payload |
|---|---|
| `tag_created` | Full tag snapshot |
| `tag_updated` | Tag ID + changed fields |
| `tag_deleted` | Tag ID + last snapshot |
| `node_tagged` | Node ID + Tag ID |
| `node_untagged` | Node ID + Tag ID |

---

## Graph Operations

### Traversal

All traversal uses SQLite `WITH RECURSIVE` CTEs. The Go code constructs the appropriate SQL based on parameters:

```go
type TraverseInput struct {
    FromID    string   // starting node UUID
    Direction string   // "outgoing" | "incoming" | "both"
    EdgeTypes []string // filter by edge type(s), empty = all
    MaxDepth  int      // 0 = unlimited (with cycle protection)
}

type TraverseResult struct {
    RootID string       // starting node ID
    Nodes  []model.Node // all reached nodes
    Edges  []model.Edge // all traversed edges
    Depth  int          // actual depth reached
}
```

### Validation

Validation runs at two points:
1. **Edge creation** — check for cycles in `depends_on` / `blocks` edges
2. **Proposal commit** — re-validate all operations against current graph state

Validations:
- **Cycle detection** — for directional edge types (`depends_on`, `blocks`), adding an edge A→B is rejected if B→...→A already exists
- **Referential integrity** — edges must reference existing nodes (enforced by FK constraints + Go checks)
- **Type constraints** — node types and edge types must be from the registered set (enforced by CHECK constraints + Go validation)
- **Duplicate edge check** — no two edges of the same type between the same pair of nodes

---

## Key Interfaces

These are the primary service interfaces that the CLI layer depends on:

```go
// GraphService handles node and edge CRUD + graph queries.
type GraphService interface {
    // Nodes
    CreateNode(ctx context.Context, input model.CreateNodeInput) (model.Node, error)
    GetNode(ctx context.Context, id string) (model.Node, error)
    ListNodes(ctx context.Context, filter model.NodeFilter) ([]model.Node, error)
    UpdateNode(ctx context.Context, id string, input model.UpdateNodeInput) (model.Node, error)
    DeleteNode(ctx context.Context, id string) error

    // Edges
    CreateEdge(ctx context.Context, input model.CreateEdgeInput) (model.Edge, error)
    GetEdge(ctx context.Context, id string) (model.Edge, error)
    ListEdges(ctx context.Context, filter model.EdgeFilter) ([]model.Edge, error)
    DeleteEdge(ctx context.Context, id string) error

    // Tags on nodes
    TagNode(ctx context.Context, nodeID string, tagID string) error
    UntagNode(ctx context.Context, nodeID string, tagID string) error

    // Graph-level
    Query(ctx context.Context, input model.GraphQueryInput) (model.GraphQueryResult, error)
    Traverse(ctx context.Context, input model.GraphTraverseInput) (model.GraphTraverseResult, error)
    Stats(ctx context.Context) (model.GraphStats, error)
}

// TagService handles tag CRUD and search.
type TagService interface {
    Create(ctx context.Context, input model.CreateTagInput) (model.Tag, error)
    Get(ctx context.Context, id string) (model.Tag, error)
    List(ctx context.Context, filter model.TagFilter) ([]model.Tag, error)
    Update(ctx context.Context, id string, input model.UpdateTagInput) (model.Tag, error)
    Delete(ctx context.Context, id string) error
    Search(ctx context.Context, keyword string) ([]model.Tag, error)
}

// ProposalService handles the proposal lifecycle.
type ProposalService interface {
    Create(ctx context.Context, input model.CreateProposalInput) (model.Proposal, error)
    Get(ctx context.Context, id string) (model.Proposal, error)
    List(ctx context.Context, filter model.ProposalFilter) ([]model.Proposal, error)
    Commit(ctx context.Context, id string) (model.ProposalCommitResult, error)
    Reject(ctx context.Context, id string) error
}

// EventService handles the append-only event log.
type EventService interface {
    Append(ctx context.Context, tx *sql.Tx, event model.Event) error
    List(ctx context.Context, filter model.EventFilter) ([]model.Event, error)
}
```

### Why interfaces?

- CLI layer depends on interfaces, not concrete implementations
- Testable: services can be mocked in CLI tests
- Clear contract: each service's capability is explicit

### Concrete implementations

```go
// In internal/graph/store.go
type Store struct {
    db     *sql.DB
    events event.Service  // for appending events on writes
}

func NewStore(db *sql.DB, events event.Service) *Store { ... }

// In internal/proposal/service.go
type Service struct {
    db    *sql.DB
    graph graph.Service   // for re-validation at commit time
    events event.Service  // for appending events
}

func NewService(db *sql.DB, graph graph.Service, events event.Service) *Service { ... }
```

---

## Initialization & Lifecycle

```go
// cmd/gm/main.go (simplified)
func main() {
    // 1. Determine DB path (--db flag or default .graphmind/graph.db)
    dbPath := resolveDBPath()

    // 2. Open database connection + apply PRAGMAs
    database, err := db.Open(dbPath)

    // 3. Run migrations (embedded SQL, auto-applied)
    db.Migrate(database)

    // 4. Wire up services
    eventSvc  := event.NewStore(database)
    graphSvc  := graph.NewStore(database, eventSvc)
    proposalSvc := proposal.NewService(database, graphSvc, eventSvc)

    // 5. Build CLI commands and execute
    root := cli.NewRoot(graphSvc, proposalSvc, eventSvc)
    root.Execute()
}
```

### Lifecycle rules

- Database connection opened once, shared across all services
- Migrations run automatically before any command executes
- `gm init` creates the `.graphmind/` directory and empty database
- All other commands expect the database to exist (exit with error if not)
- No background goroutines. No daemons. Process starts, executes one command, exits

---

## Error Propagation

```
SQLite error (constraint violation, IO error, ...)
  │
  ▼
Service layer — wraps with context, maps to domain error
  │  fmt.Errorf("create node: %w", ErrConflict)
  ▼
CLI layer — maps domain error to exit code + JSON error
  │  ErrNotFound    → exit 2, {ok: false, error: {code: "NOT_FOUND", ...}}
  │  ErrConflict    → exit 3, {ok: false, error: {code: "CONFLICT", ...}}
  │  ErrInvalidInput → exit 1, {ok: false, error: {code: "INVALID_INPUT", ...}}
  │  (unexpected)   → exit 10, {ok: false, error: {code: "INTERNAL", ...}}
  ▼
stdout (JSON) + exit code
```

The CLI layer is the **only place** that formats output. Service layer never writes to stdout/stderr.

---

## SQLite-Specific Architecture Decisions

### Single connection pool, WAL mode

```go
func Open(path string) (*sql.DB, error) {
    db, err := sql.Open("sqlite", path)
    if err != nil {
        return nil, err
    }

    // Single writer, multiple readers via WAL
    db.SetMaxOpenConns(1)  // SQLite only supports one writer at a time
    db.SetMaxIdleConns(1)
    db.SetConnMaxLifetime(0) // keep alive forever

    // Apply PRAGMAs on each connection
    // (using ConnInitHook or executing after Open)
    pragmas := []string{
        "PRAGMA journal_mode = WAL",
        "PRAGMA foreign_keys = ON",
        "PRAGMA busy_timeout = 5000",
        "PRAGMA synchronous = NORMAL",
        "PRAGMA cache_size = -64000",
        "PRAGMA temp_store = MEMORY",
    }
    for _, p := range pragmas {
        if _, err := db.Exec(p); err != nil {
            return nil, fmt.Errorf("pragma %q: %w", p, err)
        }
    }

    return db, nil
}
```

Why `MaxOpenConns(1)`?
- SQLite allows only one writer at a time
- For a CLI that runs one command and exits, concurrency is not a concern
- Avoids `SQLITE_BUSY` errors entirely

### Embedded migrations

```go
import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS

func Migrate(db *sql.DB) error {
    // Read all .sql files in order
    // For each, check if version already applied (schema_version table)
    // If not, execute within a transaction and record the version
}
```

### JSON operations

SQLite's JSON functions are used at the query level only:

```sql
-- Filter by JSON property
SELECT * FROM nodes WHERE json_extract(properties, '$.assignee') = ?;

-- Extract for listing
SELECT id, title, json_extract(properties, '$.priority') as priority FROM nodes;
```

All JSON construction and validation happens in Go. SQLite receives pre-validated JSON strings.

---

## Document Map

| Document | Scope |
|---|---|
| [README](../README.md) | What & why — vision, architecture overview, workflow |
| [Architecture](architecture.md) | **This document** — system design, data flow, interfaces |
| [Technical Design](technical-design.md) | Storage decisions, data model, SQL schema |
| [CLI Specification](cli-spec.md) | Command reference, AI-friendly design, I/O format |
| [Go & DB Conventions](go-and-db-conventions.md) | Coding standards, naming, migrations, testing |
