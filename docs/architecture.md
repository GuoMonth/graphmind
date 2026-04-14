# Architecture

System structure and data flow. For design rationale, see [Design](design.md).

---

## System Layers

```
+----------------------------------------------------------+
|                    CLI Layer (cobra)                       |
|  Parse flags/stdin -> dispatch to services -> JSON out    |
+----------------------------------------------------------+
|                   Service Layer                            |
|  GraphService  TagService  ProposalService  EventService  |
+----------------------------------------------------------+
|                   Domain Model                             |
|  Node  Edge  Tag  Event  Proposal (pure Go structs)       |
+----------------------------------------------------------+
|                   Database Layer                            |
|  Connection management  PRAGMAs  Migrations                |
+----------------------------------------------------------+
|                  SQLite (single file)                       |
|  WAL mode  Foreign keys  JSON functions  Recursive CTEs    |
+----------------------------------------------------------+
```

| Layer | Can depend on | Cannot depend on |
|---|---|---|
| CLI | Service, Model | Database directly |
| Service | Model, Database | CLI |
| Model | Nothing (stdlib only) | Any internal package |
| Database | Nothing (stdlib + driver) | Any internal package |

---

## Package Architecture

```
cmd/gm/main.go -> internal/cli -> internal/{graph, tag, proposal, event}
                                            |
                                     internal/model  (no deps)
                                     internal/db     (no internal deps)
```

### Dependency rules

```
cli -> graph, tag, proposal, event
graph -> model, db, event
tag -> model, db, event
proposal -> model, db, event, graph, tag
event -> model, db
model -> (nothing)
db -> (nothing)
```

No cycles. `proposal` depends on `graph` and `tag` for re-validation at commit time.

### Package responsibilities

| Package | Responsibility |
|---|---|
| `cmd/gm` | Entrypoint only. No business logic |
| `internal/cli` | Cobra commands. Parse flags, call services, format JSON output |
| `internal/model` | Domain types. No database imports |
| `internal/db` | Connection management, PRAGMAs, migrations |
| `internal/graph` | Node/edge CRUD, traversal, validation, cycle detection |
| `internal/tag` | Tag CRUD, FTS5 search |
| `internal/proposal` | Proposal create, validate, commit, reject |
| `internal/event` | Event append and query |

---

## Data Flow

### Read path

```
AI Agent -> CLI (parse flags) -> Service (SQL query) -> SQLite -> JSON out
```

Reads go directly against projection tables. No event replay.

### Write path

```
AI Agent -> CLI (parse stdin) -> Service ->
  BEGIN TX
    1. Validate input
    2. Mutate projection table
    3. Append event
  COMMIT TX
-> JSON out
```

Projection and event store always in sync -- single transaction guarantees it.

### Proposal flow (primary write path)

```
1. gm proposal create -> validate operations -> store as pending
2. AI Agent presents to human -> human confirms
3. gm proposal commit ->
     BEGIN TX
       Re-validate against current graph state
       For each operation: apply to projection + append event
       Mark proposal as committed
     COMMIT TX
```

Atomic: all operations succeed or none do.

---

## Validation

Runs at two points:

1. **Edge creation** -- cycle detection for directional edge types
2. **Proposal commit** -- re-validate all operations against current graph state

Checks performed:
- **Cycle detection** — recursive CTE with visited-node tracking
- **Referential integrity** — edges reference existing nodes
- **Duplicate edge** — no two edges of same type between same node pair

---

## Error Propagation

```
SQLite error
  -> Service layer: wrap with context, map to domain error
  -> CLI layer: map to exit code + JSON error envelope
  -> stdout + exit code
```

The CLI layer is the **only place** that formats output. Services never write to stdout/stderr.

---

## Initialization

1. Resolve DB path (`--db` flag or default `.graphmind/graph.db`)
2. Open SQLite connection + apply PRAGMAs
3. Run migrations (embedded SQL, auto-applied)
4. Wire services (event -> graph -> tag -> proposal -> CLI)
5. Execute one command, exit

No background goroutines. No daemons. Single command per invocation.
