# CLI Specification

Unix-native commands. Pipeline composability. AI-first I/O.

---

## Philosophy

GraphMind CLI borrows **Unix command names** — `ls`, `cat`, `grep`, `tree`, `ln`, `rm`. If you know Unix, you know `gm`. Commands compose via `|` pipes using JSONL as the interchange protocol.

The CLI is built for AI agents. Output is structured JSON. Input is flags (reads) or stdin JSON (writes). No interactive prompts, no ANSI escapes.

### Core contract

1. **stdout is always valid JSON** (envelope or JSONL) or empty with `--quiet`
2. **stderr is for diagnostics only** — never parsed
3. **Exit code is the truth** — check exit code first, then parse stdout
4. **Reads never mutate** — no side effects without explicit write commands
5. **All writes create proposals** — `commit` applies, `reject` discards

### Naming: Unix verbs, internal vocabulary

CLI commands use Unix standard names (`ls`, `rm`, `cat`). Internal types, event names, and JSON fields use the [canonical vocabulary](conventions.md) (`list`, `delete`, `get`). This is the same pattern as Unix: the command is `rm`, the syscall is `unlink()`.

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

### JSONL mode (pipe or `--jsonl`)

One JSON object per line. Every object includes `id` and `entity` for downstream commands:

```
{"id":"019abc...","entity":"node","type":"event","title":"Had dinner with David"}
{"id":"019def...","entity":"node","type":"event","title":"Met Lisa at conference"}
```

**Auto-detection**: if stdout is a pipe, use JSONL. Override with `--envelope` or `--jsonl`.

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

| Command | Unix analog | Purpose |
|---|---|---|
| `gm ls [entity]` | `ls` | List entities with filters |
| `gm cat <id>` | `cat` | Show full detail of one entity |
| `gm grep <pattern> [entity]` | `grep` | Full-text search (FTS5) |
| `gm find` | `find` | Advanced query — tags + filters + expand |
| `gm tree <id>` | `tree` | Traverse graph as tree |
| `gm log` | `git log` | View event history |
| `gm stat` | `stat` | Graph statistics |

### Write — mutate the graph (all writes create proposals)

| Command | Unix analog | Purpose |
|---|---|---|
| `gm add` | `touch` | Create node → proposal |
| `gm ln <from-id> <to-id>` | `ln` | Create edge → proposal |
| `gm tag <node-id> <tag-name>` | `tag` (macOS) | Tag a node (upsert tag) → proposal |
| `gm untag <node-id> <tag-name>` | — | Remove tag from node → proposal |
| `gm mv <id>` | `mv` | Update entity → proposal |
| `gm rm <id>...` | `rm` | Delete entities → proposal |
| `gm batch` | `xargs` | Multi-operation proposal from stdin |
| `gm commit <proposal-id>` | `git commit` | Commit a pending proposal |
| `gm reject <proposal-id>` | `git reset` | Reject a pending proposal |

### Organize — maintain graph health

| Command | Unix analog | Purpose |
|---|---|---|
| `gm merge <tag-id> <tag-id>` | — | Merge duplicate tags → proposal |
| `gm gc` | `git gc` | Find orphan tags, disconnected nodes |

### Utility

| Command | Purpose |
|---|---|
| `gm init` | Initialize graph database |
| `gm schema` | Machine-readable command/type schema (JSON) |

---

## Pipeline Model

Commands compose via `|` pipes. **Read commands filter, write commands batch.**

### Protocol

- Pipe output: JSONL (one JSON object per line, each with `id` + `entity`)
- Pipe input: commands detect piped stdin and use IDs as context
- No pipe: commands use flags/positional args as normal

### Per-command pipe behavior

| Command | Standalone | With piped input |
|---|---|---|
| `gm ls` | List all (with filters) | Filter: only list entities matching piped IDs |
| `gm cat` | Show one by ID | Show detail for each piped entity |
| `gm grep` | Search entire graph | Filter: search only within piped entities |
| `gm find` | Query full graph | Use piped entities as starting points |
| `gm tree` | Tree from one root | Use each piped entity as tree root |
| `gm log` | All events | Events for piped entities only |
| `gm stat` | Overall graph stats | Stats for piped entities only |
| `gm rm` | Delete by ID args | Delete all piped entities → proposal |
| `gm tag` | Tag one node | Tag all piped entities → proposal |
| `gm mv` | Update one entity | Update all piped entities → proposal |

### Pipeline examples

```bash
# Find events about travel and show their relationship tree
gm ls node --type event | gm grep "travel" | gm tree --depth 2

# Tag all events matching a pattern
gm ls node | gm grep "Bangkok" | gm tag --name "thailand-trip"
gm commit <proposal-id>

# Delete all archived events
gm grep "archived" | gm rm
gm commit <proposal-id>

# Show event log for nodes with a specific tag
gm find --tag "startup-idea" | gm log

# List orphan tags (0 associated nodes)
gm ls tag --orphan
```

---

## Command Reference

### gm init

Initialize a new graph database.

```
gm init [--db <path>]
```

Creates the database file and runs all migrations. Safe to run on existing databases (migrations are idempotent).

---

### gm schema

Output machine-readable schema as JSON. AI agents call this once at session start for self-discovery.

```
gm schema
```

Returns: all commands, parameters, input/output schemas, type registries.

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
| `--status <status>` | Filter by status |
| `--tag <name>` | Filter nodes by tag name |
| `--orphan` | Tags with 0 nodes, or nodes with 0 edges |
| `--limit <n>` | Max results (default 50) |
| `--after <cursor>` | Cursor for pagination |

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
gm cat <id> [--expand <n>]
```

| Flag | Description |
|---|---|
| `--expand <n>` | Include nodes and edges within N hops (neighborhood expansion) |

Auto-detects entity type from the ID. Returns the full entity object including all properties.

```bash
gm cat 019abc-...                  # show node detail
gm cat 019abc-... --expand 2      # node + 2-hop neighborhood
```

---

### gm grep <pattern> [entity]

Full-text search across the graph using FTS5.

```
gm grep <pattern> [node|edge|tag] [flags]
```

Searches titles, descriptions, and property values. Entity filter is optional — defaults to searching all entities.

| Flag | Description |
|---|---|
| `--limit <n>` | Max results |
| `--after <cursor>` | Cursor for pagination |

```bash
gm grep "payment"              # search everything
gm grep "payment" node         # search only nodes
gm grep "API endpoint" tag     # search tag names and descriptions
```

---

### gm find

Advanced multi-modal query. The power search command.

```
gm find [flags]
```

Combines tag matching, type filtering, and neighborhood expansion in one call.

| Flag | Description |
|---|---|
| `--tag <name>` | Find nodes with this tag (repeatable) |
| `--type <type>` | Filter by node type |
| `--status <status>` | Filter by status |
| `--text <pattern>` | FTS5 text filter |
| `--expand <n>` | Expand N hops from matched nodes |
| `--limit <n>` | Max results |
| `--after <cursor>` | Cursor for pagination |

Primary AI agent pattern: find anchor nodes, load surrounding context.

```bash
gm find --tag "thailand-trip" --type event --expand 2
gm find --tag "startup-idea" --tag "David" --expand 1    # nodes with both tags
gm find --text "conference" --status ongoing
```

---

### gm tree <id>

Traverse the graph from a root node, displayed as a tree.

```
gm tree <id> [flags]
```

| Flag | Description |
|---|---|
| `--depth <n>` | Max traversal depth (default 3) |
| `--type <edge-type>` | Only follow edges of this type |
| `--direction <dir>` | `outgoing` (default), `incoming`, `both` |

```bash
gm tree 019abc-...                              # default tree
gm tree 019abc-... --type caused_by --depth 5   # causal chain
gm tree 019abc-... --direction incoming          # what caused this?
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

### gm stat

Graph statistics overview.

```
gm stat [--entity-id <id>]
```

Without arguments: total counts (nodes by type, edges by type, tags, events). With `--entity-id`: stats for a specific entity (edge count, tag count, event count).

---

### gm add

Create an event node. Returns a pending proposal.

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
| `--property <key=value>` | Set a property (repeatable) |

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

Returns: proposal object with proposal ID and one `create_node` operation.

```bash
gm add --type event --title "Had dinner with David" --who "David" --where "Bangkok Kitchen"
echo '{"type":"thought","title":"Consider switching to Rust","description":"..."}' | gm add
```

---

### gm ln <from-id> <to-id>

Create a directed edge between two entities. Returns a pending proposal.

Auto-detects whether the IDs belong to nodes or tags. Both IDs must be the same entity type (both nodes or both tags).

```
gm ln <from-id> <to-id> --type <edge-type>
```

| Flag | Description |
|---|---|
| `--type <type>` | Edge type — open string (required) |
| `--property <key=value>` | Set a property (repeatable) |

**Node edges** (event-to-event relationships):
```bash
gm ln 019abc-... 019def-... --type caused_by
gm ln 019abc-... 019def-... --type followed_by --property "confidence=high"
```

**Tag edges** (concept-to-concept relationships):
```bash
gm ln <tag-id> <tag-id> --type parent_of
gm ln <tag-id> <tag-id> --type synonym_of
gm ln <tag-id> <tag-id> --type related_to
```

---

### gm tag <node-id> <tag-name>

Associate a tag with a node. If the tag doesn't exist, it is created (upsert). Returns a pending proposal.

```
gm tag <node-id> <tag-name> [--description <text>]
```

| Flag | Description |
|---|---|
| `--description <text>` | Tag description (used on creation or update) |

Pipe mode: tag all piped entities with the given tag.

```bash
gm tag 019abc-... "payment"
gm tag 019abc-... "payment" --description "Payment processing subsystem"

# Pipe: tag all matching nodes
gm ls node --type task | gm grep "billing" | gm tag --name "payment"
```

---

### gm untag <node-id> <tag-name>

Remove a tag association from a node. Returns a pending proposal.

```
gm untag <node-id> <tag-name>
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
| `--property <key=value>` | Set a property (repeatable) |

Stdin JSON: partial object — only provided fields are updated.

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

Auto-detects entity type. Deleting a node also deletes its edges and tag associations (cascade). Multiple IDs create a single proposal with multiple operations.

```bash
gm rm 019abc-...
gm rm 019abc-... 019def-... 019ghi-...

# Pipe: delete all matching entities
gm grep "deprecated" | gm rm
```

---

### gm batch

Create a multi-operation proposal from stdin JSON. The primary way to make complex atomic changes.

```
echo '<json>' | gm batch
```

Stdin format: JSON array of operations. Each operation has a `command` and `data` field.

Within a batch, operations can reference entities created by earlier operations using `reference` (zero-based index into the operations array) instead of `id`.

```json
[
  {"command": "add", "data": {"type": "event", "title": "Met David at conference", "who": "David", "where": "Tech Summit 2026"}},
  {"command": "add", "data": {"type": "person", "title": "David Chen", "description": "Startup founder, met at conference"}},
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

Re-validates all operations against the current graph state before applying. If the graph has changed in a way that makes an operation invalid, the entire commit is rejected.

---

### gm reject <proposal-id>

Reject a pending proposal. Discards all operations.

```
gm reject <proposal-id>
```

---

### gm merge <tag-id> <tag-id>

Merge two tags into one. Returns a pending proposal.

```
gm merge <tag-id> <tag-id> [--keep first|second]
```

The `--keep` flag determines which tag survives (default: `first`). All node associations from the removed tag are transferred to the surviving tag.

---

### gm gc

Find orphan entities — tags with 0 node associations, nodes with 0 edges. Read-only; reports findings but does not create proposals.

```
gm gc [--entity node|tag]
```

Pipe the output to `gm rm` to create a cleanup proposal:

```bash
gm gc --entity tag | gm rm
gm commit <proposal-id>
```

---

## Proposal Flow

All write commands (`add`, `ln`, `tag`, `untag`, `mv`, `rm`, `batch`, `merge`) create **pending proposals**. No direct graph mutation.

```
Write command → validate → create pending proposal → return proposal
Human confirms → AI calls gm commit → re-validate → apply atomically
Human rejects → AI calls gm reject → discard
```

A write command's response includes the proposal ID and a summary of operations:

```json
{
  "ok": true,
  "data": {
    "proposal_id": "019abc-...",
    "status": "pending",
    "operations": [
      {"action": "create_node", "entity": "node", "summary": "event: Had dinner with David"}
    ]
  }
}
```

---

## Global Flags

| Flag | Default | Description |
|---|---|---|
| `--db <path>` | `.graphmind/graph.db` | Path to SQLite database |
| `--quiet` | `false` | Suppress stdout, exit code only |
| `--pretty` | `false` | Pretty-print JSON |
| `--jsonl` | auto | Force JSONL output |
| `--envelope` | auto | Force envelope output |

---

## Open Type System

Node types and edge types are **open strings** — not enumerated, not validated. The AI agent decides what types to use based on context.

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

The AI agent is free to invent new types as needed. Consistency is encouraged through `next_steps` hints, not enforced through validation.
