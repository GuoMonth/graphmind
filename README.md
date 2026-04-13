# GraphMind

**Graph-based project management, built natively for AI agents.**

GraphMind is a local-first CLI that AI agents ([Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://openai.com/index/codex/), [Copilot](https://github.com/features/copilot)) use to read and write a project graph stored in SQLite. Humans talk to the AI agent. The AI agent calls `gm`.

> _"I just describe what's happening, and the system figures out what I need to do."_

---

## Quick Start

```bash
# Initialize a project
gm init

# Create a task (returns a pending proposal)
gm add --type task --title "Design auth module" --description "JWT-based authentication"

# Link nodes with a typed edge
gm ln <from-id> <to-id> --type depends_on

# Tag a node (creates the tag if it doesn't exist)
gm tag <node-id> "backend"

# Commit a proposal — applies all operations atomically
gm commit <proposal-id>

# List entities
gm ls node                         # all nodes
gm ls node --type task             # only tasks
gm ls edge --type depends_on         # edges by type
gm ls tag                          # all tags

# Show full detail of any entity
gm cat <id>
```

All commands output JSON envelopes (`{"ok": true, "data": ...}`), making them composable in Unix pipelines.

---

## Core Commands

| Command | Description |
|---------|-------------|
| `gm init` | Initialize project database |
| `gm add` | Create a node (task, epic, decision, risk, release, discussion) |
| `gm ln` | Create a directed edge between two nodes |
| `gm tag` | Associate a tag with a node (upsert) |
| `gm commit` | Apply a pending proposal atomically |
| `gm reject` | Discard a pending proposal |
| `gm ls` | List entities with type/status filters and pagination |
| `gm cat` | Show full detail of one entity by ID |

### Proposal-First Writes

Every write operation creates a **pending proposal** rather than modifying data directly. This gives humans (or AI agents) a chance to review before committing:

```
gm add --type task --title "Fix login bug"
  → proposal created (pending)

gm commit <proposal-id>
  → all operations applied atomically in one SQLite transaction
```

---

## Why

Traditional tools (Linear, Jira) flatten projects into forms, statuses, and boards. The simplification helps humans, but **the storage discards the real structure**.

Real projects are **dynamically evolving graphs** — multiple node types (tasks, decisions, risks), multiple relationship types (depends-on, blocks, decomposes-into), continuously changing. GraphMind preserves the full graph. AI agents handle the complexity.

---

## How It Works

```
Human
  |  natural language
AI Agent (Claude Code / Codex / Copilot)
  |  structured JSON
GraphMind CLI (gm)
  |  read / write
Graph (SQLite)
```

1. Human describes what's happening
2. AI agent asks follow-up questions
3. AI agent queries the graph for context (`gm ls`, `gm cat`)
4. AI agent creates a proposal with nodes, edges, and tags (`gm add`, `gm ln`, `gm tag`)
5. Human confirms, AI agent commits (`gm commit`)
6. Repeat as the project evolves

---

## Three-Layer Association Model

AI agents discover relationships through three complementary layers:

| Layer | Mechanism | Cost | Purpose |
|---|---|---|---|
| **Tags** | Shared named concepts | Low | Discovery entry point — O(N) implicit clustering |
| **Edges** | Typed directed relationships | High | Structural analysis — depends-on, blocks, decomposes |
| **AI Semantic** | Content reasoning at query time | Zero | Deep association on small subgraphs |

Tags are the search funnel entry point. AI agents extract 2–5 tags per node, creating implicit connections without O(N²) explicit edges. See [Design](docs/design.md) for the full rationale.

---

## Design Principles

| Principle | Meaning |
|---|---|
| **Graph-first** | Store the real structure, never flatten at the storage layer |
| **Proposal-first** | All writes staged as proposals, committed after human confirmation |
| **Event-sourced** | All mutations recorded as events; current state is a projection |
| **Tags as semantic bridge** | AI-extracted concepts link related nodes without explicit edges |
| **CLI-as-Tool** | For AI agents, not humans. JSON I/O, semantic exit codes |
| **Local-first** | SQLite, zero config, single-user first |

---

## Documentation

| Document | Scope |
|---|---|
| [Design](docs/design.md) | Why — core thesis, tag system, event sourcing, storage choice |
| [Architecture](docs/architecture.md) | What — system layers, packages, data flow |
| [CLI Specification](docs/cli-spec.md) | API — command contract, type registries |
| [Conventions](docs/conventions.md) | Rules — naming, Go, database, engineering workflow |

---

## Tech Stack

| | |
|---|---|
| Language | Go 1.25 |
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
