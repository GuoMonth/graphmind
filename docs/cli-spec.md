# CLI Specification

## AI-Friendly First

GraphMind CLI is built **for AI agents, not for humans**. Every design decision optimizes for machine consumption. Humans interact through AI agents (Claude Code, Codex, Copilot), which call `gm` behind the scenes.

### What "AI-Friendly" means concretely

| Principle | Traditional (human) CLI | GraphMind (AI-agent) CLI |
|---|---|---|
| **Output** | Colored text, tables, spinners | Structured JSON, always |
| **Input** | Interactive prompts, wizards | Flags + stdin JSON, never interactive |
| **Errors** | Natural language messages | Typed error codes + messages |
| **Discoverability** | `--help` for humans to read | `gm schema` outputs machine-readable JSON |
| **Confirmation** | "Are you sure? [y/N]" | Never. The AI agent handles confirmation with the human |
| **Pagination** | `less`, scrolling | Cursor-based, explicit `--limit` / `--after` |
| **Idempotency** | Not guaranteed | Idempotent where possible. Clearly documented when not |
| **Formatting** | ANSI colors, box drawing | No ANSI escapes. Plain JSON to stdout, plain text errors to stderr |
| **Exit codes** | 0 or 1 | Semantic: 0 success, 1 input error, 2 not found, 3 conflict, 10 internal |

### Core contract with AI agents

1. **stdout is always valid JSON** (or empty on `--quiet`). AI agents can `json.Unmarshal` every response.
2. **stderr is for diagnostics only** — never parsed, never relied upon.
3. **Exit code is the truth** — AI agents check exit code first, then parse JSON.
4. **No side effects without explicit commands** — reading never mutates state.
5. **Stdin JSON for complex input** — no escaping hell, no multiline flag values.

---

## Output Envelope

Every command returns a JSON envelope to stdout:

### Success

```json
{
  "ok": true,
  "data": { ... }
}
```

### Error

```json
{
  "ok": false,
  "error": {
    "code": "NOT_FOUND",
    "message": "Node 0192d4e5-7a2b-7000-8000-000000000001 not found"
  }
}
```

### Error codes

| Code | Exit code | Meaning |
|---|---|---|
| `INVALID_INPUT` | 1 | Malformed JSON, missing required field, bad UUID |
| `NOT_FOUND` | 2 | Referenced node/edge/proposal does not exist |
| `CONFLICT` | 3 | Duplicate, cycle detected, proposal already committed |
| `INVALID_STATE` | 3 | Operation not valid in current state (e.g., commit a rejected proposal) |
| `INTERNAL` | 10 | Unexpected error (db corruption, IO failure) |

---

## Command Structure

```
gm <resource> <action> [flags] [< stdin]
```

### Resources and actions

| Resource | Actions | Description |
|---|---|---|
| `node` | `create`, `get`, `list`, `update`, `delete` | Graph nodes |
| `edge` | `create`, `get`, `list`, `delete` | Graph edges (relationships) |
| `proposal` | `create`, `get`, `list`, `commit`, `reject` | Staged change proposals |
| `event` | `list` | Event log (read-only) |
| `graph` | `query`, `traverse`, `stats` | Graph-level operations |
| `schema` | *(no action needed)* | Output CLI schema as JSON |
| `init` | *(no action needed)* | Initialize a new GraphMind database |

### Global flags

| Flag | Default | Description |
|---|---|---|
| `--db <path>` | `.graphmind/graph.db` | Path to SQLite database file |
| `--quiet` | `false` | Suppress stdout. Only exit code |
| `--pretty` | `false` | Pretty-print JSON (for human debugging only) |

---

## Commands in Detail

### `gm init`

Initialize a new GraphMind database in the current directory.

```bash
$ gm init
```

```json
{
  "ok": true,
  "data": {
    "db_path": ".graphmind/graph.db",
    "created": true
  }
}
```

---

### `gm node create`

Create a node. Input via stdin JSON.

```bash
$ echo '{"type":"task","title":"Payment API design","properties":{"assignee":"Alice"}}' | gm node create
```

```json
{
  "ok": true,
  "data": {
    "id": "0192d4e5-7a2b-7000-8000-000000000001",
    "type": "task",
    "title": "Payment API design",
    "status": "open",
    "properties": { "assignee": "Alice" },
    "created_at": "2026-04-02T07:00:00Z",
    "updated_at": "2026-04-02T07:00:00Z"
  }
}
```

### `gm node get`

```bash
$ gm node get --id 0192d4e5-7a2b-7000-8000-000000000001
```

### `gm node list`

```bash
$ gm node list
$ gm node list --type task
$ gm node list --status open --limit 20
$ gm node list --filter '$.assignee == "Alice"'
```

### `gm node update`

Partial update via stdin JSON. Only provided fields are changed.

```bash
$ echo '{"status":"done","properties":{"completed_at":"2026-04-05T12:00:00Z"}}' | gm node update --id <uuid>
```

### `gm node delete`

```bash
$ gm node delete --id <uuid>
```

Deleting a node also removes all edges connected to it.

---

### `gm edge create`

```bash
$ echo '{"type":"depends_on","from_id":"<uuid>","to_id":"<uuid>"}' | gm edge create
```

Returns error with code `CONFLICT` if this would create a circular dependency (checked at creation time).

### `gm edge list`

```bash
$ gm edge list --from <uuid>
$ gm edge list --to <uuid>
$ gm edge list --type depends_on
$ gm edge list --node <uuid>          # edges in either direction
```

### `gm edge delete`

```bash
$ gm edge delete --id <uuid>
```

---

### `gm proposal create`

Stage a batch of changes as a proposal. Input via stdin JSON.

```bash
$ cat << 'EOF' | gm proposal create
{
  "description": "Extract payment module into microservice",
  "operations": [
    {"action": "create_node", "data": {"type": "epic", "title": "Payment microservice extraction"}},
    {"action": "create_node", "data": {"type": "task", "title": "Payment API design", "properties": {"assignee": "Alice"}}},
    {"action": "create_node", "data": {"type": "task", "title": "Payment data migration", "properties": {"assignee": "Bob"}}},
    {"action": "create_edge", "data": {"type": "decompose", "from_ref": 0, "to_ref": 1}},
    {"action": "create_edge", "data": {"type": "decompose", "from_ref": 0, "to_ref": 2}},
    {"action": "create_edge", "data": {"type": "depends_on", "from_ref": 2, "to_id": "0192d4e5-7a2b-7000-8000-00000000002a"}}
  ]
}
EOF
```

**Note:** `from_ref` / `to_ref` reference nodes by their index in the `operations` array (for nodes created within the same proposal). `from_id` / `to_id` reference existing nodes by UUID.

```json
{
  "ok": true,
  "data": {
    "id": "0192d4e5-8c3d-7000-8000-000000000010",
    "status": "pending",
    "description": "Extract payment module into microservice",
    "operations_count": 6,
    "preview": {
      "nodes_to_create": 3,
      "edges_to_create": 3,
      "nodes_to_update": 0,
      "nodes_to_delete": 0
    },
    "created_at": "2026-04-02T07:15:00Z"
  }
}
```

### `gm proposal get`

```bash
$ gm proposal get --id <uuid>
```

Returns full proposal with all operations and their details.

### `gm proposal list`

```bash
$ gm proposal list
$ gm proposal list --status pending
```

### `gm proposal commit`

Commit a pending proposal — apply all operations to the graph.

```bash
$ gm proposal commit --id <uuid>
```

This is atomic: either all operations succeed or none do. On success, corresponding events are recorded.

### `gm proposal reject`

Reject a pending proposal.

```bash
$ gm proposal reject --id <uuid>
```

---

### `gm graph query`

Search nodes and edges by keyword or property.

```bash
$ gm graph query --keyword "payment"
$ gm graph query --filter '$.assignee == "Alice"' --type task
```

### `gm graph traverse`

Traverse the graph from a starting node.

```bash
$ gm graph traverse --from <uuid> --direction out --edge-type depends_on --depth 3
```

```json
{
  "ok": true,
  "data": {
    "root": "0192d4e5-7a2b-7000-8000-000000000001",
    "nodes": [ ... ],
    "edges": [ ... ],
    "depth_reached": 2
  }
}
```

### `gm graph stats`

Return graph-level statistics.

```bash
$ gm graph stats
```

```json
{
  "ok": true,
  "data": {
    "node_count": 42,
    "edge_count": 67,
    "node_types": { "task": 30, "epic": 5, "decision": 4, "risk": 3 },
    "edge_types": { "depends_on": 25, "decompose": 20, "blocks": 12, "caused_by": 10 },
    "open_proposals": 2
  }
}
```

---

### `gm event list`

Query the event log.

```bash
$ gm event list --limit 50
$ gm event list --type node_created
$ gm event list --since 2026-04-01T00:00:00Z
```

---

### `gm schema`

Output the full CLI schema as machine-readable JSON. This allows an AI agent to **discover** all available commands, their parameters, and expected input/output formats at runtime.

```bash
$ gm schema
```

```json
{
  "ok": true,
  "data": {
    "version": "0.1.0",
    "commands": [
      {
        "name": "node create",
        "description": "Create a new graph node",
        "input": "stdin",
        "input_schema": {
          "type": "object",
          "required": ["type", "title"],
          "properties": {
            "type": { "type": "string", "enum": ["task", "epic", "decision", "risk", "release", "discussion"] },
            "title": { "type": "string" },
            "status": { "type": "string", "default": "open" },
            "properties": { "type": "object" }
          }
        },
        "flags": [],
        "output_schema": { "$ref": "#/definitions/node" }
      }
    ]
  }
}
```

AI agents call `gm schema` once at the start of a session to learn the full API surface, then construct commands accordingly. This is the **self-describing** nature of the CLI.

---

## Input Conventions

### Simple lookups: use flags

```bash
$ gm node get --id <uuid>
$ gm node list --type task --status open
```

### Mutations: use stdin JSON

```bash
$ echo '{"type":"task","title":"Design API"}' | gm node create
$ echo '{"status":"done"}' | gm node update --id <uuid>
```

### Why stdin over flags for mutations

- No shell escaping issues for complex values
- Supports nested objects naturally
- AI agents construct JSON trivially
- Consistent: the AI agent always sends JSON, always receives JSON

---

## Edge Type Registry

Built-in edge types (extensible):

| Edge type | Semantics | Example |
|---|---|---|
| `depends_on` | A cannot start until B is done | "Migration depends on API stable" |
| `blocks` | A is preventing B from progressing | "Bug #12 blocks Release v2" |
| `decompose` | A is broken down into B | "Epic decomposes into Task" |
| `caused_by` | A was caused by B | "Hotfix caused by deploy failure" |
| `related_to` | A is related to B (weak link) | "Discussion related to Decision" |
| `supersedes` | A replaces B | "New plan supersedes old plan" |

---

## Node Type Registry

Built-in node types (extensible):

| Node type | Semantics |
|---|---|
| `task` | A unit of work to be completed |
| `epic` | A large body of work decomposed into tasks |
| `decision` | A decision that was made or needs to be made |
| `risk` | An identified risk or concern |
| `release` | A release or milestone |
| `discussion` | An ongoing discussion or open question |
