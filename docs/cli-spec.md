# CLI Specification

## AI-Friendly-First

GraphMind CLI is built for AI agents, not humans.

| Principle | Implementation |
|---|---|
| Output | Structured JSON to stdout, always |
| Input | Flags + stdin JSON, never interactive |
| Errors | Typed error codes + structured messages |
| Discoverability | `gm schema` outputs machine-readable JSON |
| Confirmation | Never. AI agent handles this with human |
| Pagination | Cursor-based: `--limit` / `--after` |
| Formatting | No ANSI escapes. Plain JSON only |
| Exit codes | Semantic: 0 success, 1 input error, 2 not found, 3 conflict, 10 internal |

### Core contract

1. **stdout is always valid JSON** (or empty with `--quiet`)
2. **stderr is for diagnostics only** -- never parsed
3. **Exit code is the truth** -- check exit code first, then parse JSON
4. **No side effects without explicit commands** -- reads never mutate
5. **Stdin JSON for complex input** -- no escaping issues

---

## Output Envelope

Every command wraps its response:

- Success: `{"ok": true, "data": {...}}`
- Error: `{"ok": false, "error": {"code": "...", "message": "..."}}`

### Error codes

| Code | Exit | Meaning |
|---|---|---|
| `INVALID_INPUT` | 1 | Malformed JSON, missing required field, bad UUID |
| `NOT_FOUND` | 2 | Referenced entity does not exist |
| `CONFLICT` | 3 | Duplicate, cycle detected, proposal already committed |
| `INVALID_STATE` | 3 | Operation not valid in current state |
| `INTERNAL` | 10 | Unexpected error |

---

## Command Structure

```
gm <resource> <action> [flags] [< stdin]
```

### Resource / action matrix

| Resource | Actions |
|---|---|
| `node` | `create`, `get`, `list`, `update`, `delete`, `tag`, `untag` |
| `edge` | `create`, `get`, `list`, `delete` |
| `tag` | `create`, `get`, `list`, `update`, `delete`, `search` |
| `proposal` | `create`, `get`, `list`, `commit`, `reject` |
| `event` | `list` |
| `graph` | `query`, `traverse`, `stats` |
| `schema` | *(no action)* |
| `init` | *(no action)* |

### Global flags

| Flag | Default | Description |
|---|---|---|
| `--db <path>` | `.graphmind/graph.db` | Path to SQLite database |
| `--quiet` | `false` | Suppress stdout, exit code only |
| `--pretty` | `false` | Pretty-print JSON |

---

## Input Conventions

- **Reads**: use flags -- `gm node get --id <uuid>`, `gm node list --type task`
- **Writes**: use stdin JSON -- `echo '{...}' | gm node create`

Why stdin for writes: no shell escaping issues, supports nested objects, AI agents construct JSON trivially.

---

## Key Command Patterns

**Neighborhood expansion** -- `gm graph query` supports `--expand N` to retrieve matched nodes plus all nodes and edges within N hops. Primary AI agent pattern: find anchor, load context in one call.

**Proposal internal references** -- in `gm proposal create`, operations can reference nodes created within the same proposal via `from_reference` / `to_reference` (index into operations array), alongside `from_id` / `to_id` for existing nodes.

**Self-describing schema** -- `gm schema` outputs the full CLI schema as machine-readable JSON (all commands, parameters, input/output schemas). AI agents call this once at session start.

---

## Type Registries

### Node types

| Type | Semantics |
|---|---|
| `task` | A unit of work |
| `epic` | A large body of work decomposed into tasks |
| `decision` | A decision made or to be made |
| `risk` | An identified risk or concern |
| `release` | A release or milestone |
| `discussion` | An ongoing discussion or open question |

### Edge types

| Type | Semantics |
|---|---|
| `depends_on` | A cannot start until B is done |
| `blocks` | A is preventing B from progressing |
| `decompose` | A is broken down into B |
| `caused_by` | A was caused by B |
| `related_to` | Weak link between A and B |
| `supersedes` | A replaces B |

Both registries are extensible.
