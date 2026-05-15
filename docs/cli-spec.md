# CLI Specification

Unix-native commands. Pipeline composability. AI-first I/O.

> **Status key** — commands and flags are marked **[implemented]** (shipped) or
> **[planned]** (not yet available). AI agents and scripts should only rely on
> implemented items.

---

## Philosophy

GraphMind CLI borrows **Unix command names** — `ls`, `cat`, `grep`, `ln`, `rm`.
If you know Unix, you know `gm`. Commands compose via `|` pipes.
The CLI is built for AI agents. Output is structured JSON. Input is flags
(reads) or stdin JSON (writes). No interactive prompts, no ANSI escapes.

### Core contract

1. **stdout is always valid JSON** (envelope) or empty with `--quiet`
2. **stderr is for diagnostics only** — never parsed
3. **Exit code is the truth** — check exit code first, then parse stdout
4. **Reads never mutate** — no side effects without explicit write commands
5. **All writes create proposals** — `commit` applies, `reject` discards

### Naming: Unix verbs, internal vocabulary

CLI commands use Unix standard names (`ls`, `rm`, `cat`). Internal types, event
names, and JSON fields use the [canonical vocabulary](conventions.md) (`list`,
`delete`, `get`). This is the same pattern as Unix: the command is `rm`, the
syscall is `unlink()`.

| CLI command | Internal action | Event type |
|---|---|---|
| `gm add` | `create` | `node_created` |
| `gm rm` | `delete` | `node_deleted` |
| `gm mv` | `update` | `node_updated` |
| `gm ls` | `list` | — |
| `gm cat` | `get` | — |

---

## Output Protocol

### Envelope mode (default)

```json
{"ok": true, "data": {...}}
{"ok": false, "error": {"code": "NOT_FOUND", "message": "..."}}
```

Used when AI agents call `gm` directly. Full response in one JSON object.

### Error codes

| Code | Exit | Meaning |
|---|---|---|
| `INVALID_INPUT` | 1 | Malformed JSON, missing required field, bad UUID |
| `NOT_FOUND` | 2 | Referenced entity does not exist |
| `CONFLICT` | 3 | Duplicate, cycle detected, proposal already committed |
| `INVALID_STATE` | 3 | Operation not valid in current state |
| `INTERNAL` | 10 | Unexpected error |

---

## Command Map

### Read — query the graph

| Command | Unix analog | Purpose | Status |
|---|---|---|---|
| `gm ls [entity]` | `ls` | List entities with filters | **implemented** |
| `gm cat <id>` | `cat` | Show full detail of one entity | **implemented** |
| `gm grep <pattern>` | `grep` | Full-text node search (FTS5) | **implemented** |
| `gm log` | `git log` | View event history | **implemented** |
| `gm find` | `find` | Advanced query — tags + filters + expand | planned |
| `gm tree <id>` | `tree` | Traverse graph as tree | planned |
| `gm stat` | `stat` | Graph statistics | planned |

### Write — mutate the graph (all writes create proposals)

| Command | Unix analog | Purpose | Status |
|---|---|---|---|
| `gm add` | `touch` | Create node → proposal | **implemented** |
| `gm ln <from-id> <to-id>` | `ln` | Create edge → proposal | **implemented** |
| `gm tag <node-id> <tag-name>` | `tag` (macOS) | Tag a node (upsert tag) → proposal | **implemented** |
| `gm mv <id>` | `mv` | Update entity → proposal | **implemented** |
| `gm rm <id>...` | `rm` | Delete entities → proposal | **implemented** |
| `gm batch` | `xargs` | Multi-operation proposal from stdin | **implemented** |
| `gm commit <proposal-id>` | `git commit` | Commit a pending proposal | **implemented** |
| `gm reject <proposal-id>` | `git reset` | Reject a pending proposal | **implemented** |
| `gm untag <node-id> <tag-name>` | — | Remove tag from node → proposal | planned |

### Organize — maintain graph health

| Command | Unix analog | Purpose | Status |
|---|---|---|---|
| `gm merge <tag-id> <tag-id>` | — | Merge duplicate tags → proposal | planned |
| `gm gc` | `git gc` | Find orphan tags, disconnected nodes | planned |

### Utility

| Command | Purpose | Status |
|---|---|---|
| `gm init` | Initialize graph database | **implemented** |
| `gm update check` | Check GitHub Releases for a newer CLI version | **implemented** |
| `gm update apply` | Download and install a release archive | **implemented** |
| `gm schema` | Machine-readable command/type schema (JSON) | planned |

---

## Pipeline Model

**Read commands** output a JSON envelope. The `gm rm` command reads JSONL IDs
from a piped stdin to build a batch delete proposal. Other commands accept
positional args or flags.

### Pipeline examples

```bash
# Delete all nodes matching a search
gm grep "deprecated" | gm rm
gm commit <proposal-id>

# Stage multiple changes in one atomic proposal
echo '[{"command":"add","data":{"type":"event","title":"Design review"}},
       {"command":"add","data":{"type":"event","title":"Implementation start"}},
       {"command":"ln","data":{"type":"followed_by","from_reference":0,"to_reference":1}}]' \
  | gm batch
gm commit <proposal-id>
```

---

## Command Reference

### gm init

Initialize a new graph database.

```
gm init [--db <path>]
```

Creates the database file and runs all migrations. Safe to run on existing
databases (migrations are idempotent).

---

### gm update check

Check GitHub Releases for a newer `gm` binary.

```
gm update check
```

This command performs a blocking network request and refreshes the local update
cache immediately. The selected platform asset must include a valid
GitHub-provided `sha256:` digest or the check fails.

Regular `gm` commands may also trigger a **non-blocking** background update
check at most once every 24 hours. Any update hint is written to **stderr** so
stdout remains valid JSON.

---

### gm update apply

Download and install the latest matching release asset for the current
platform. `gm` verifies the selected asset's `sha256:` digest before replacing
the current executable.

```
gm update apply [--version <tag>]
```

| Flag | Description |
|---|---|
| `--version <tag>` | Install a specific release tag instead of the latest one |

Examples:

```bash
gm update apply
gm update apply --version v0.3.1
```

---

### gm ls [entity]

List entities with filters.

```
gm ls [node|edge|tag|tag_edge|proposal] [flags]
```

Entity defaults to `node` when omitted.

| Flag | Description |
|---|---|
| `--type <type>` | Filter by type (node type or edge type) |
| `--status <status>` | Filter by status (applies to `node` and `proposal`) |
| `--limit <n>` | Max results (default 50) |
| `--after <cursor>` | Cursor for pagination (pass the last item's `id`; for tags, pass the last `name`) |

```bash
gm ls                          # list nodes (default)
gm ls node --type event        # list event nodes
gm ls edge --type caused_by    # list causal edges
gm ls tag                      # list all tags
gm ls tag_edge                 # list tag-to-tag edges
gm ls tag_edge --type parent_of  # list hierarchical tag relationships
gm ls proposal --status pending  # list pending proposals
```

---

### gm cat <id>

Show full detail of one entity by ID.

```
gm cat <id>
```

Auto-detects entity type by trying lookups in order: node → edge → tag →
tag\_edge → proposal. Returns the full entity object including all properties.

```bash
gm cat 019abc-...   # show any entity by ID
```

---

### gm grep <pattern>

Full-text search across nodes using FTS5.

```
gm grep <pattern> [flags]
```

Searches node titles and descriptions using SQLite FTS5. Supports boolean
queries (AND/OR/NOT) and phrase matching.

| Flag | Description |
|---|---|
| `--limit <n>` | Max results (default 50) |
| `--after <cursor>` | Cursor for pagination |

```bash
gm grep "payment"              # simple keyword
gm grep '"fix login"'          # exact phrase
gm grep "auth AND token"       # boolean AND
gm grep "pay*"                 # prefix wildcard
```

---

### gm log

View event history.

```
gm log [flags]
```

| Flag | Description |
|---|---|
| `--entity-id <id>` | Events for a specific entity |
| `--type <event-type>` | Filter by event type (e.g. `node_created`) |
| `--since <duration>` | Events within duration (e.g. `24h`, `7d`) |
| `--limit <n>` | Max results (default 50) |
| `--after <cursor>` | Cursor for pagination |

```bash
gm log                             # recent events
gm log --entity-id 019abc-...     # history of one entity
gm log --type node_created --since 7d
```

---

### gm add

Create a node. Returns a pending proposal.

```
echo '<json>' | gm add
gm add --type <type> --title <title> [flags]
```

Input via stdin JSON (complex) or flags (simple):

| Flag | Description |
|---|---|
| `--type <type>` | Node type — open string, AI decides (required) |
| `--title <title>` | Brief summary (required) |
| `--description <text>` | Full narrative |
| `--who <text>` | People involved |
| `--where <text>` | Location |
| `--event-time <text>` | When it happened (free-form: "2026-04-12", "last Tuesday", "summer 2025") |
| `--status <status>` | Initial status |

Stdin JSON format:

```json
{
  "type": "event",
  "title": "Had dinner with David",
  "description": "Met at the Thai restaurant near the office, discussed the startup idea",
  "who": "David, Lisa",
  "where": "Bangkok Kitchen, 3rd Ave",
  "event_time": "2026-04-12",
  "properties": {"mood": "happy", "importance": "high"}
}
```

Returns: proposal object with one `create_node` operation.

```bash
gm add --type event --title "Had dinner with David" --who "David" --where "Bangkok Kitchen"
echo '{"type":"thought","title":"Consider switching to Rust","description":"..."}' | gm add
```

---

### gm ln <from-id> <to-id>

Create a directed edge between two entities. Returns a pending proposal.

Auto-detects whether the IDs belong to nodes or tags. Both IDs must be the same
entity type (both nodes or both tags).

```
gm ln <from-id> <to-id> --type <edge-type>
```

| Flag | Description |
|---|---|
| `--type <type>` | Edge type — open string (required) |

**Node edges** (node-to-node relationships):
```bash
gm ln 019abc-... 019def-... --type caused_by
gm ln 019abc-... 019def-... --type followed_by
```

**Tag edges** (concept-to-concept relationships):
```bash
gm ln <tag-id> <tag-id> --type parent_of
gm ln <tag-id> <tag-id> --type synonym_of
gm ln <tag-id> <tag-id> --type related_to
```

---

### gm tag <node-id> <tag-name>

Associate a tag with a node. If the tag doesn't exist, it is created (upsert).
Returns a pending proposal.

```
gm tag <node-id> <tag-name> [--description <text>]
```

| Flag | Description |
|---|---|
| `--description <text>` | Tag description (used on creation) |

```bash
gm tag 019abc-... "payment"
gm tag 019abc-... "payment" --description "Payment processing subsystem"
```

---

### gm mv <id>

Update a node's fields. Returns a pending proposal.

```
echo '<json>' | gm mv <id>
gm mv <id> [flags]
```

| Flag | Description |
|---|---|
| `--title <title>` | New title |
| `--description <text>` | New description |
| `--who <text>` | New people involved |
| `--where <text>` | New location |
| `--event-time <text>` | New event time (free-form) |
| `--status <status>` | New status |
| `--type <type>` | New type |

Stdin JSON: partial object — only provided fields are updated. Properties are
merged: new keys are added, existing keys overwritten, unmentioned keys kept.

```bash
gm mv 019abc-... --status resolved
gm mv 019abc-... --who "David, Lisa, James" --where "Office"
echo '{"event_time":"2026-04-14","properties":{"follow_up":"true"}}' | gm mv 019abc-...
```

---

### gm rm <id>...

Delete one or more entities. Returns a pending proposal.

```
gm rm <id> [<id>...]
```

Auto-detects entity type (node, edge, or tag\_edge). Deleting a node also
deletes its edges and tag associations (cascade). Multiple IDs create a single
proposal with multiple operations.

Accepts JSONL from stdin (`{"id":"..."}` per line) in addition to positional
args.

```bash
gm rm 019abc-...
gm rm 019abc-... 019def-... 019ghi-...

# Pipe: delete all matching entities
gm grep "deprecated" | gm rm
```

---

### gm batch

Create a multi-operation proposal from stdin JSON. The primary way to make
complex atomic changes.

```
echo '<json>' | gm batch
```

Stdin format: JSON array of operations. Each operation has a `command` and
`data` field. Valid commands: `add`, `ln`, `tag`, `mv`, `rm`.

Within a batch, operations can reference entities created by earlier operations
using `reference` (zero-based index into the operations array) instead of `id`.

```json
[
  {"command": "add", "data": {"type": "event", "title": "Met David at conference", "who": "David"}},
  {"command": "add", "data": {"type": "person", "title": "David Chen"}},
  {"command": "ln", "data": {"type": "involves", "from_reference": 0, "to_reference": 1}},
  {"command": "tag", "data": {"reference": 0, "tag_name": "networking"}},
  {"command": "tag", "data": {"reference": 1, "tag_name": "networking"}}
]
```

Returns: proposal object with all operations.

---

### gm commit <proposal-id>

Commit a pending proposal. Applies all operations atomically.

```
gm commit <proposal-id>
```

Re-validates all operations against the current graph state before applying.
If any operation fails (cycle detection, missing entity, etc.), the entire
commit is rolled back and the graph is unchanged.

---

### gm reject <proposal-id>

Reject a pending proposal. Discards all operations.

```
gm reject <proposal-id>
```

---

## Planned Commands

The following commands are designed but **not yet implemented**. Do not use
them in scripts or agent loops — they will return a "command not found" error.

### gm find *(planned)*

Advanced multi-modal query combining tag matching, type filtering, and
neighborhood expansion.

```
gm find --tag <name> [--type <type>] [--status <status>] [--text <pattern>] [--expand <n>]
```

### gm tree <id> *(planned)*

Traverse the graph from a root node up to `--depth` hops.

```
gm tree <id> [--depth <n>] [--type <edge-type>] [--direction outgoing|incoming|both]
```

### gm stat *(planned)*

Graph statistics overview: total counts by type, orphan detection.

```
gm stat [--entity-id <id>]
```

### gm untag <node-id> <tag-name> *(planned)*

Remove a tag association from a node. Returns a pending proposal.

```
gm untag <node-id> <tag-name>
```

### gm merge <tag-id> <tag-id> *(planned)*

Merge two tags into one. Returns a pending proposal.

```
gm merge <tag-id> <tag-id> [--keep first|second]
```

### gm gc *(planned)*

Find orphan entities (tags with 0 node associations, nodes with 0 edges).

```
gm gc [--entity node|tag]
```

### gm schema *(planned)*

Output machine-readable command/type schema as JSON for AI agent self-discovery.

```
gm schema
```

---

## Proposal Flow

All write commands (`add`, `ln`, `tag`, `mv`, `rm`, `batch`) create **pending
proposals**. No direct graph mutation occurs.

```
Write command → validate → create pending proposal → return proposal
Human confirms → AI calls gm commit → re-validate → apply atomically
Human rejects → AI calls gm reject → discard
```

A write command's response includes the proposal ID and a summary of
operations:

```json
{
  "ok": true,
  "data": {
    "id": "019abc-...",
    "status": "pending",
    "operations": [
      {"action": "create_node", "entity": "node", "summary": "event: Had dinner with David"}
    ],
    "created_at": "2026-05-15T10:30:00.000Z",
    "updated_at": "2026-05-15T10:30:00.000Z"
  }
}
```

---

## Global Flags

| Flag | Default | Description |
|---|---|---|
| `--db <path>` | `.graphmind/graph.db` | Path to SQLite database |
| `--quiet` | `false` | Suppress stdout; rely on exit code only |
| `--pretty` | `false` | Pretty-print JSON output |

---

## Open Type System

Node types and edge types are **open strings** — not enumerated, not validated.
The AI agent decides what types to use based on context.

### Node type examples (not exhaustive)

| Type | Use when |
|---|---|
| `event` | Something that happened ("Had dinner with David") |
| `person` | A person who appears in events ("David Chen") |
| `place` | A location that recurs ("Bangkok Kitchen") |
| `thought` | An idea, reflection, or realization |
| `meeting` | A scheduled gathering |
| `observation` | Something noticed or perceived |
| `decision` | A decision made or to be made |

### Edge type examples — node edges (not exhaustive)

| Type | Use when |
|---|---|
| `caused_by` | A was caused by B |
| `followed_by` | A happened after B (temporal chain) |
| `related_to` | Weak link between A and B |
| `involves` | A involves person/place B |
| `reminded_by` | A reminded someone of B |
| `contradicts` | A conflicts with B |
| `supersedes` | A replaces B |

### Edge type examples — tag edges (not exhaustive)

| Type | Use when |
|---|---|
| `parent_of` | A is a broader concept than B (hierarchy) |
| `synonym_of` | A and B are the same concept, different names |
| `related_to` | A and B are conceptually related |
| `opposite_of` | A and B are opposing concepts |

The AI agent is free to invent new types as needed. Consistency is encouraged
through `next_steps` hints, not enforced through validation.
