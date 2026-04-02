# Design

Why GraphMind is designed the way it is.

---

## Core Thesis

Real projects are dynamically evolving graphs -- multiple node types (tasks, decisions, risks), multiple relationship types (depends-on, blocks, decomposes-into), continuously changing as understanding evolves.

Traditional tools (Linear, Jira) flatten this into forms + statuses + boards. The simplification helps humans operate the tool, but **the storage layer discards the real structure**. Once lost, it cannot be recovered.

GraphMind preserves the full graph. AI agents handle the complexity. Humans see simplified views produced by the AI agent -- but the underlying truth remains intact.

---

## The Relationship Discovery Problem

As a project graph grows, a fundamental question emerges:

> Given new information, how does an AI agent find existing nodes it relates to?

**Explicit edges only** -- create typed edges between every related pair. Fatal flaw: O(N squared) cost. Every new node must be compared against every existing node. In practice, only strong, obvious relationships get edges. Weaker but important connections are silently lost.

**Full-text search only** -- use FTS5 keyword matching. Fatal flaw: lexical blindness. "Payment API" won't match "billing endpoint". Results are a flat list with no structural context.

**Tags as semantic bridge** -- introduce a shared vocabulary layer. Named concepts that multiple nodes reference. Two nodes sharing a tag are implicitly related without an explicit edge. This is what GraphMind chooses.

---

## Three-Layer Association Model

Three complementary layers, from coarse to precise:

| Layer | Mechanism | Cost | Signal | Purpose |
|---|---|---|---|---|
| **Tags** | Shared named concepts | Low (AI auto-extracts) | Medium (thematic) | Discovery entry point |
| **Edges** | Typed directed relationships | High (infer type + direction) | Strong (structural) | Dependency analysis |
| **AI Semantic** | Content reasoning at query time | Zero (no storage) | Broad but weak | Deep association |

No single layer is sufficient alone. They form a **search funnel**:

1. **Tags** -- narrow from entire graph to a thematic cluster (tens of nodes)
2. **Edges** -- within the cluster, trace structural relationships
3. **AI** -- on the small subgraph, reason about implications

---

## Tag System

### What tags are

A tag is a **named concept** that recurs across a project -- a theme, domain area, component, or concern. Tags have a name (concise label) and description (rich context for AI reasoning and FTS5 search breadth).

Nodes sharing a tag are implicitly related. Tagging N nodes costs O(N) operations, compared to O(N squared) explicit edges.

### Tags vs edges

| | Tags | Edges |
|---|---|---|
| Express | "Same topic" (symmetric) | "Depends on / blocks" (directed, typed) |
| Creation cost | Low (AI extracts from content) | High (infer type + direction + both endpoints) |
| Scaling | O(N) | O(N squared) |

Complementary, not competing. Tags enable cheap discovery. Edges enable precise structural reasoning.

### Tag lifecycle

1. **Extraction** -- AI extracts candidate concepts from user input
2. **Deduplication** -- AI searches existing tags before creating new ones
3. **Association** -- tags attached to nodes
4. **Enrichment** -- tag descriptions evolve, becoming living documentation
5. **Maintenance** -- merge synonyms, split overloaded tags, retire orphans (all via proposals)

### Design decisions

- **Flat, not hierarchical** -- hierarchies impose a single classification axis; real projects have overlapping dimensions
- **Separate table, not JSON properties** -- indexed queries, FTS5 integration, event-sourced, cross-node JOINs
- **Junction table has no metadata** -- event log provides full history
- **2-5 tags per node** -- fewer loses signal, more creates noise

### Limitations

| Limitation | Mitigation |
|---|---|
| No directionality | By design -- tags discover, edges structure |
| Synonym proliferation | AI must search before creating; periodic merge via proposals |
| Stale tags | Query orphaned tags; propose cleanup |
| Over-tagging | Behavioral guideline: 2-5 tags per node |
| No weighted relationships | Edges with properties handle strength |
| Depends on AI quality | FTS5 as fallback; humans correct via proposals |

---

## Event Sourcing

All mutations are recorded as immutable events. Current state (nodes, edges, tags) is derived by projection.

**Why:**
- Full audit trail of every change
- Reconstruct state at any point in time
- Evolution analysis -- how project understanding changed over time

**Inline projection** -- event and projection update in the same SQLite transaction. No async consumers, no eventual consistency. Simple and guaranteed consistent.

**Rebuild capability** -- if projections corrupt, replay all events to reconstruct. Disaster recovery path, not normal operation.

---

## Proposal-First Write Model

AI agents don't write directly to the graph. All changes are staged as proposals, committed after human confirmation.

**Why:**
- Prevents bad AI modeling from polluting the graph
- Atomic -- all operations in a proposal succeed or none do
- Re-validated at commit time (graph may have changed)
- Human stays in control of the truth

---

## Storage: SQLite

Single SQLite database file. No external server.

**Why SQLite:**
- Local-first, zero config -- single file, no network
- OLTP fit -- CRUD + event appending are transactional workloads
- Graph traversal -- recursive CTEs handle dependency chains at project scale
- JSON columns -- json_extract() for flexible properties
- Pure Go driver -- no CGO dependency

**Why not others:**

| Alternative | Rejected because |
|---|---|
| Neo4j, DGraph | Requires standalone server -- violates local-first |
| PostgreSQL | Not zero-config single-machine |
| DuckDB | OLAP-oriented, not OLTP |
| Plain JSON files | No queries, no transactions |

DuckDB could be added later as a read-only analytics layer (it reads SQLite directly).
