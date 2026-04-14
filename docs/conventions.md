# Conventions

Rules for naming, coding, and engineering workflow. All contributors (human and AI) must follow these.

---

## Naming

The `gm` CLI is consumed by AI agents. **One concept = one word, everywhere.** CLI flags, Go code, DB schema, JSON fields, documentation -- all use the same vocabulary.

### Canonical entities

| Word | Meaning |
|---|---|
| `node` | A memory in the graph — an event, person, place, thought, or any recorded entity |
| `edge` | A directed relationship between two nodes (causal, temporal, associative) |
| `tag` | An AI-constructed semantic label for thematic clustering |
| `proposal` | A staged batch of changes awaiting confirmation |
| `event` | An immutable record of a past mutation (system concept, not user-facing "memory") |
| `graph` | The overall structure (virtual, not a DB table) |

### Canonical actions (internal vocabulary)

Used in Go code, event types, JSON fields, and database. CLI commands use Unix aliases (see below).

| Word | Meaning |
|---|---|
| `create` | Make a new entity |
| `get` | Retrieve one entity by ID |
| `list` | Retrieve multiple entities with filters |
| `update` | Partial modification |
| `delete` | Remove permanently |
| `search` | Full-text keyword search (FTS5) |
| `query` | Multi-modal search (keyword + tag + filter + expand) |
| `traverse` | Walk the graph following edges |
| `commit` / `reject` | Apply or discard a pending proposal |
| `tag` / `untag` | Associate or remove a tag on a node |

### CLI command names (Unix aliases)

CLI commands use Unix standard names for zero learning curve. Internal vocabulary stays canonical.

| CLI verb | Internal action | Unix analog |
|---|---|---|
| `add` | `create` | `touch` |
| `cat` | `get` | `cat` |
| `ls` | `list` | `ls` |
| `mv` | `update` | `mv` |
| `rm` | `delete` | `rm` |
| `grep` | `search` | `grep` |
| `find` | `query` | `find` |
| `tree` | `traverse` | `tree` |
| `ln` | `create` (edge) | `ln` |
| `log` | `list` (events) | `git log` |

### Banned synonyms

| Never use | Always use |
|---|---|
| item, entity, vertex | `node` |
| link, connection, relation | `edge` |
| label, category, topic | `tag` |
| draft, changeset, batch | `proposal` |
| add, new, insert | `create` |
| remove, destroy, drop | `delete` |
| fetch, read, show, find (single) | `get` |
| modify, change, edit, patch | `update` |
| attributes, metadata, fields | `properties` |
| body, content, data (for events) | `payload` |
| source_id, origin_id | `from_id` |
| target_id, destination_id | `to_id` |
| state, phase, stage | `status` |
| kind, class, sort | `type` |

### Flag suffix rules

- Flags accepting a UUID **must** end with `-id`: `--id`, `--from-id`, `--to-id`, `--node-id`, `--tag-id`
- Flags accepting a name **must** end with `-name`: `--tag-name`
- A flag without `-id` suffix **must not** accept a UUID

### Direction values

Full words only: `outgoing`, `incoming`, `both`. Never `out`, `in`, `all`.

### No-abbreviation rule

Use complete English words. Allowed exceptions: `id`, `ctx`, `db`, `tx`, `err`, `ok`, `fts`.

Everything else spelled out: `reference` not `ref`, `description` not `desc`, `properties` not `props`.

### Naming patterns

- CLI commands: `gm <verb> [entity|id] [--flag-name value]`
- Event types: `{entity}_{past_tense_verb}` -- e.g., `node_created`, `proposal_committed`
- Proposal operations: `{verb}_{entity}` -- e.g., `create_node`, `tag_node`
- Go types: `{Action}{Entity}{Suffix}` -- e.g., `CreateNodeInput`, `GraphQueryResult`
- Go acronyms: all-caps -- `ID`, `UUID`, `NodeID`, `FTS`, `JSON`, `SQL`

---

## Go

### Hard rules

- **Go 1.26** -- minimum and target
- **No CGO** -- single static binary
- **No ORM** -- `database/sql` directly
- **No `any`** -- banned by default. Allowed only for: JSON properties (`map[string]any`), `database/sql` scanning, generic constraints, third-party interfaces. Every use must have a justifying comment

### Style

- `gofmt` and `go vet` -- mandatory, no exceptions
- `context.Context` as first parameter for all service functions
- Return `(result, error)`, never `panic` for business logic
- No globals except sentinel errors
- JSON struct tags: `snake_case`
- Imports grouped: stdlib, third-party, internal (blank line between groups)

### Dependencies

| Package | Purpose |
|---|---|
| `modernc.org/sqlite` | SQLite driver (pure Go) |
| `github.com/google/uuid` | UUID v7 generation |
| `github.com/spf13/cobra` | CLI framework |

Minimize dependencies. Every new one must justify itself.

### Error handling

- Standard `error` interface, no custom frameworks
- Wrap with `fmt.Errorf("context: %w", err)`
- Sentinel errors: `ErrNotFound`, `ErrConflict`, `ErrInvalidInput`, `ErrInvalidState`
- CLI layer maps domain errors to exit codes

### Testing

- Standard `testing` package, no third-party frameworks
- Table-driven tests with subtests
- In-memory SQLite (`:memory:`) for database tests
- Coverage targets: service layer >= 80%, overall >= 60%

---

## Database

### SQLite connection

Every connection applies PRAGMAs: WAL mode, foreign keys ON, busy timeout 5s, synchronous NORMAL, 64MB cache, temp store in memory.

### Data type mapping

| Go type | SQLite type | Notes |
|---|---|---|
| UUID | `TEXT` | Lowercase hyphenated string |
| Timestamps | `TEXT` | ISO 8601, always UTC |
| JSON | `TEXT` | Validated in Go, queryable via `json_extract()` |
| Open types | `TEXT` | Node type, edge type — no CHECK constraint, no enum in Go |
| Free-form time | `TEXT` | `event_time` — stored as-is, may be fuzzy ("last Tuesday") |

### Schema conventions

- Tables: plural, snake_case (`nodes`, `edges`)
- Columns: singular, snake_case (`created_at`, `from_id`)
- Indexes: `idx_{table}_{column}`
- Foreign keys end with `_id`
- `properties` defaults to '{}', never `NULL`

### UUID v7

All primary keys use UUID v7 (RFC 9562). Time-ordered for natural chronological indexing. Generated in Go, stored as TEXT.

### Transactions

- All writes inside explicit transactions
- Proposal commit is atomic
- Reads don't require transactions (WAL mode)

### Migrations

- Embedded SQL via Go `embed`
- Sequential numbering: `001_init.sql`, `002_...`
- Forward-only, no rollbacks
- Auto-applied at startup
- Tracked via `schema_version` table

---

## Engineering Workflow

### Design, Code, Test

Every feature follows three phases. No phase skipped.

1. **Design** -- write or update docs, define interfaces and data models
2. **Code** -- implement against design, run `make check` continuously
3. **Test** -- every public function has tests, table-driven with subtests

### Quality gates

| Metric | Threshold |
|---|---|
| Cyclomatic complexity | <= 15 |
| Function length | <= 80 lines |
| Line length | <= 140 chars |
| File size (target / hard limit) | <= 300 / <= 500 lines |

### Git hooks

- **Pre-commit**: `go fmt` + `go vet` + `golangci-lint` (blocks commit)
- **Pre-push**: `go build` + `go test -race` (blocks push)
- Setup: `make setup-hooks`

### Commit discipline

- One logical change per commit
- Imperative mood, 72-char subject line
- `Co-authored-by` trailer for AI-assisted commits

### Key Makefile targets

`make build`, `make test`, `make lint`, `make check` (pre-commit gate), `make validate` (pre-push gate), `make setup-hooks`.
