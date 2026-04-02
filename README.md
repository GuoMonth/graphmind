# GraphMind

**Graph-based project management, built natively for AI agents.**

GraphMind is a local-first project management CLI designed for AI agents like [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [Codex](https://openai.com/index/codex/), and [Copilot](https://github.com/features/copilot). Humans never touch GraphMind directly — they talk to an AI agent, and the AI agent uses GraphMind to read and write the graph.

> _"I just describe what's happening, and the system figures out what I need to do."_

---

## Why

Traditional project management tools (Linear, Jira) simplify projects into forms, statuses, and boards so humans can operate them directly. Simplification for presentation is fine — **but the underlying storage should not throw away the real structure**.

Real projects have **many node types** (tasks, decisions, risks, releases), **many relationship types** (depends-on, blocks, decomposes-into, caused-by), and they **evolve continuously** as understanding changes. At their core, they are **dynamically evolving graphs**.

GraphMind preserves the full graph underneath. AI agents process the complexity and present it to humans in digestible form.

---

## Architecture

```
Human
  ↕  natural language conversation
AI Agent (Claude Code / Codex / Copilot)
  ↕  structured commands + JSON
GraphMind CLI (gm)
  ↕  read / write
Graph (SQLite)
```

| Layer | Responsibility |
|---|---|
| **Human** | Provide context, make decisions, confirm proposals |
| **AI Agent** | Converse, ask follow-ups, extract structure, generate proposals, call CLI |
| **GraphMind CLI** | Read/write interface to the graph — structured I/O, predictable behavior, machine-parseable |
| **Graph** | Store the complete, real relationships of the project as a graph |

---

## Workflow

A concrete example:

**① Human provides context**

```
User: We've decided to extract the payment module from the monolith into
      a microservice. Alice owns the API design, Bob owns the data migration.
      Starting next week — blocked on the auth service API being stable first.
```

**② AI agent asks follow-up questions**

```
AI: What's the target version for the split? Is there a hard deadline?
AI: "Blocked on auth API being stable" — what does stable mean here? Who decides?
```

**③ AI agent queries the graph via CLI**

```bash
$ gm query --related "auth API" --format json
```

```
AI: The graph already has "Auth API refactor" (#42), in progress, owned by
    Charlie. Is that the dependency you're referring to?
```

**④ AI agent generates a proposal via CLI**

```bash
$ gm proposal create --format json << 'EOF'
{
  "nodes": [
    {"title": "Payment microservice extraction", "type": "epic"},
    {"title": "Payment API design", "type": "task", "assignee": "Alice"},
    {"title": "Payment data migration", "type": "task", "assignee": "Bob"}
  ],
  "edges": [
    {"from": "Payment microservice extraction", "to": "Payment API design", "type": "decompose"},
    {"from": "Payment microservice extraction", "to": "Payment data migration", "type": "decompose"},
    {"from": "Payment data migration", "to": "#42", "type": "depends_on"}
  ]
}
EOF
```

The AI agent presents the result in human-readable form:

```
AI: Here's what I'll create:
    📦 Payment microservice extraction (epic)
     ├── Payment API design (Alice)
     └── Payment data migration (Bob) → depends on #42 Auth API refactor
    Shall I commit this?
```

**⑤ Human confirms, commit**

```bash
$ gm proposal commit <proposal-id>
```

**⑥ Continuous evolution**

```
User: Alice says the API design is done, but we discovered we need
      a new gateway layer — that wasn't planned for.
```

The AI agent calls the CLI again to update statuses, create new nodes, adjust relationships — generating a new proposal for confirmation. The cycle repeats.

---

## Design Principles

| Principle | Meaning |
|---|---|
| **Graph-first** | Project data is stored as a graph. Never flattened into forms, lists, or boards at the storage layer |
| **Proposal-first** | All changes are staged as proposals. Only committed after human confirmation. Prevents bad modeling from polluting the system |
| **Event-sourced** | All mutations are recorded as events. Current state is derived by projection. Supports retrospection and evolution analysis |
| **Evolving Graph** | The graph is never "done". Supports enrichment, correction, splitting, merging, and reclassification |
| **CLI-as-Tool** | The CLI is a tool interface for AI agents, not a UI for humans. Structured I/O, predictable behavior |
| **Local-first** | Runs locally by default (SQLite). Zero config. Single-user first |

---

## Role of AI

AI agents don't make decisions. They handle the graph complexity that humans can't manage visually:

- **Extract** — pull structured nodes and relationships from natural language
- **Link** — connect new information to existing graph nodes
- **Validate** — check graph consistency (circular deps, missing links, etc.)
- **Project** — transform complex graph relationships into human-readable views
- **Compress** — summarize large-scale graph information

> AI is a "complexity rectifier", not a "project manager".

---

## Non-goals

Not pursuing in the current phase:

- Enterprise permission systems
- Complex approval workflows
- Web UI / frontend-heavy experience
- Full replacement for Linear or Jira
- General-purpose graph database

---

## Tech Stack

| | |
|---|---|
| Language | Go 1.26 |
| Storage | SQLite (via `modernc.org/sqlite`, pure Go) |
| Primary Keys | UUID v7 |
| Interface | CLI (JSON I/O) |
| Linting | golangci-lint v2 |
| Quality gates | Git hooks (pre-commit, pre-push) |

---

## License

[MIT](LICENSE)
