# GraphMind

**Graph-based memory storage, built natively for AI agents.**

GraphMind is a local-first CLI that AI agents ([Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://openai.com/index/codex/), [Copilot](https://github.com/features/copilot)) use to record and retrieve human memories as a graph stored in SQLite. Humans describe what happened. The AI agent captures who, when, where — and builds the connections.

> _"I just describe what happened, and the system remembers it — with all the people, places, and connections."_

---

## Quick Start

```bash
# Initialize the memory graph
gm init

# Record a memory (returns a pending proposal)
gm add --type event --title "Had dinner with David" \
       --who "David, Lisa" --where "Bangkok Kitchen" --event-time "2026-04-12"

# Link memories with a typed edge
gm ln <from-id> <to-id> --type followed_by

# Tag a memory (creates the tag if it doesn't exist)
gm tag <node-id> "thailand-trip"

# Update a memory with new details
gm mv <node-id> --who "David, Lisa, James" --event-time "last Friday evening"

# Commit a proposal — applies all operations atomically
gm commit <proposal-id>

# List memories
gm ls node                         # all memories
gm ls node --type event            # only events
gm ls edge --type caused_by        # causal edges
gm ls tag                          # all tags

# Full-text search
gm grep "dinner"

# Show full detail of any entity
gm cat <id>

# View event history
gm log --since 24h

# Delete a memory (cascade removes edges and tag associations)
gm rm <node-id>
```

All commands output JSON envelopes (`{"ok": true, "data": ...}`), making them composable in Unix pipelines.

---

## Core Commands

### Write (returns a pending proposal)

| Command | Description |
|---------|-------------|
| `gm add` | Create a memory node — type, title, who, where, event_time |
| `gm ln` | Create a directed edge between two nodes |
| `gm tag` | Associate a tag with a node (upsert) |
| `gm mv` | Update a node (title, who, where, event_time, status, properties) |
| `gm rm` | Delete nodes or edges (cascade) |
| `gm batch` | Multi-operation atomic proposal from JSON stdin |

### Control (apply or discard proposals)

| Command | Description |
|---------|-------------|
| `gm commit` | Apply a pending proposal atomically |
| `gm reject` | Discard a pending proposal |

### Read (query the graph)

| Command | Description |
|---------|-------------|
| `gm ls` | List entities with type/status filters and pagination |
| `gm cat` | Show full detail of one entity by ID |
| `gm grep` | Full-text search nodes via FTS5 |
| `gm log` | View event history with time/entity filters |

### Setup

| Command | Description |
|---------|-------------|
| `gm init` | Initialize memory graph database |

### Proposal-First Writes

Every write operation creates a **pending proposal** rather than modifying data directly. This gives humans (or AI agents) a chance to review before committing:

```
gm add --type event --title "Met David at conference" --who "David" --where "Tech Summit"
  → proposal created (pending)

gm commit <proposal-id>
  → all operations applied atomically in one SQLite transaction
```

---

## Core Concepts

### Memory Nodes

Every record is a **memory** — something that happened, was observed, decided, or thought. Memories have dedicated fields for the essential context:

| Field | Purpose | Example |
|---|---|---|
| `type` | Open string — AI decides | `event`, `person`, `place`, `thought` |
| `title` | Brief summary | "Had dinner with David" |
| `who` | People involved | "David, Lisa" |
| `where` | Location | "Bangkok Kitchen, 3rd Ave" |
| `event_time` | When it happened (free-form) | "2026-04-12", "last Tuesday", "summer 2025" |

**Two timestamps, different meanings:**
- `event_time` — when the event occurred (user/AI supplied, free-form string)
- `created_at` / `updated_at` — when the system recorded the memory (auto, ISO 8601)

### Open Type System

Node types and edge types are **open strings** — not enumerated, not validated. The AI agent decides what types to use. Life doesn't fit into 6 categories.

### AI-Constructed Tags

Tags are named concepts that recur across memories (themes, people, places, projects). The AI agent extracts and manages them — humans don't tag directly. Two memories sharing a tag are implicitly related without explicit edges.

---

## Why

Traditional note-taking tools flatten memories into linear lists, folders, or databases. **The structure is lost the moment it's recorded.**

Human memory is a **graph** — people, places, events, ideas connected by causality, time, association, and meaning. GraphMind preserves the full graph. AI agents handle the complexity of organizing, connecting, and retrieving memories.

---

## How It Works

```
Human
  |  natural language
AI Agent (Claude Code / Codex / Copilot)
  |  structured JSON
GraphMind CLI (gm)
  |  read / write
Memory Graph (SQLite)
```

1. Human describes what happened
2. AI agent asks follow-up questions — who was there? when? where?
3. AI agent queries the graph for context (`gm ls`, `gm cat`, `gm grep`)
4. AI agent creates a proposal with memories, edges, and tags (`gm add`, `gm ln`, `gm tag`)
5. Human confirms, AI agent commits (`gm commit`)
6. Repeat as life happens

---

## Three-Layer Association Model

AI agents discover relationships through three complementary layers:

| Layer | Mechanism | Cost | Purpose |
|---|---|---|---|
| **Tags** | Shared named concepts | Low | Discovery entry point — O(N) implicit clustering |
| **Edges** | Typed directed relationships | High | Structural analysis — caused_by, followed_by |
| **AI Semantic** | Content reasoning at query time | Zero | Deep association on small subgraphs |

Tags are the search funnel entry point. AI agents extract 2–5 tags per memory, creating implicit connections without O(N²) explicit edges. See [Design](docs/design.md) for the full rationale.

---

## Design Principles

| Principle | Meaning |
|---|---|
| **Graph-first** | Store the real structure, never flatten at the storage layer |
| **Proposal-first** | All writes staged as proposals, committed after confirmation |
| **Event-sourced** | All mutations recorded as events; current state is a projection |
| **AI-friendly first** | Structured JSON I/O, hints, summaries, next-step guidance |
| **Open types** | Node/edge types are free strings — the AI defines the taxonomy |
| **Tags as semantic bridge** | AI-constructed concepts link related memories without explicit edges |
| **Local-first** | SQLite, zero config, single-user first |

---

## Documentation

| Document | Scope |
|---|---|
| [Design](docs/design.md) | Why — core thesis, open type system, tag system, event sourcing |
| [Architecture](docs/architecture.md) | What — system layers, packages, data flow |
| [CLI Specification](docs/cli-spec.md) | API — command contract, open type system, pipeline model |
| [Conventions](docs/conventions.md) | Rules — naming, Go, database, engineering workflow |

---

## Tech Stack

| | |
|---|---|
| Language | Go 1.26 |
| Storage | SQLite (`modernc.org/sqlite`, pure Go, no CGO) |
| Primary Keys | UUID v7 (time-ordered, RFC 9562) |
| Interface | CLI with JSON envelope protocol |
| CI/CD | GitHub Actions — lint, test, cross-compile release |
| Quality | golangci-lint v2, 45+ tests, race-clean |

---

## Install

Download a pre-built binary from [Releases](https://github.com/GuoMonth/graphmind/releases), or build from source:

```bash
go install github.com/senguoyun-guosheng/graphmind/cmd/gm@latest
```

---

## License

[MIT](LICENSE)
