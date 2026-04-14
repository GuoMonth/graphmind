# Design

Why GraphMind is designed the way it is.

---

## Core Thesis

Human memory is a graph — people, places, events, ideas connected by causality, time, association, and meaning. Traditional note-taking tools flatten this into linear lists, folders, or databases. The structure is lost the moment it's recorded.

GraphMind preserves the full graph. AI agents handle the complexity of organizing, connecting, and retrieving memories. Humans just describe what happened.

**GraphMind is a graph-structured memory store, operated by AI agents on behalf of humans.**

---

## What Gets Stored

Every record in GraphMind is a **memory node** — something that happened, was observed, decided, or thought. Memories are rich, not flat:

| Field | Purpose | Example |
|---|---|---|
| `type` | Open string — AI decides the category | `event`, `person`, `place`, `thought`, `meeting` |
| `title` | Brief summary | "Had dinner with David" |
| `description` | Full narrative | "Met at the Thai restaurant near the office..." |
| `who` | People involved | "David, Lisa" |
| `where` | Location | "Bangkok Kitchen, 3rd Ave" |
| `event_time` | When it happened (free text, can be fuzzy) | "2026-04-12", "last Tuesday", "summer 2025" |
| `status` | Optional lifecycle state | "ongoing", "resolved", "recalled" |
| `properties` | Extensible key-value pairs | `{"mood": "happy", "importance": "high"}` |

**Two timestamps, different meanings:**
- `event_time` — when the event actually occurred (user/AI supplied, free-form string)
- `created_at` / `updated_at` — when the system recorded / last modified the memory (auto, ISO 8601)

Never confuse these. `event_time` is the truth about the world. `created_at` is the truth about the record.

---

## Open Type System

GraphMind does **not** enumerate allowed types. Both node types and edge types are open strings — the AI agent decides what categories to use based on context.

**Why open, not enumerated:**
- Memory is unbounded — life doesn't fit into 6 categories
- AI agents can evolve their classification over time
- Different users/agents can develop different type vocabularies
- The system should never reject a memory because its type isn't in a whitelist

**Guidance, not enforcement:** The CLI provides hints in `next_steps` to guide AI agents toward consistent type usage, but never rejects input based on type value.

---

## The Relationship Discovery Problem

As the memory graph grows, a fundamental question emerges:

> Given new information, how does an AI agent find existing memories it relates to?

**Explicit edges only** — create typed edges between every related pair. Fatal flaw: O(N²) cost. Every new memory must be compared against every existing one. In practice, only strong, obvious relationships get edges. Weaker but important connections are silently lost.

**Full-text search only** — use FTS5 keyword matching. Fatal flaw: lexical blindness. "dinner at the Thai place" won't match "Bangkok Kitchen meal". Results are a flat list with no structural context.

**Tags as semantic bridge** — introduce a shared vocabulary layer. Named concepts that multiple memories reference. Two memories sharing a tag are implicitly related without an explicit edge. This is what GraphMind chooses.

---

## Three-Layer Association Model

Three complementary layers, from coarse to precise:

| Layer | Mechanism | Cost | Signal | Purpose |
|---|---|---|---|---|
| **Tags** | Shared named concepts | Low (AI auto-extracts) | Medium (thematic) | Discovery entry point |
| **Edges** | Typed directed relationships | High (infer type + direction) | Strong (structural) | Causal/temporal analysis |
| **AI Semantic** | Content reasoning at query time | Zero (no storage) | Broad but weak | Deep association |

No single layer is sufficient alone. They form a **search funnel**:

1. **Tags** — narrow from entire graph to a thematic cluster (tens of memories)
2. **Edges** — within the cluster, trace structural relationships (caused_by, followed_by)
3. **AI** — on the small subgraph, reason about implications and patterns

---

## Tag System

### What tags are

A tag is a **named concept** that recurs across memories — a theme, person, place, project, emotion, or any recurring idea. Tags have a name (concise label) and description (rich context for AI reasoning and FTS5 search).

Memories sharing a tag are implicitly related. Tagging N memories costs O(N) operations, compared to O(N²) explicit edges.

### Tags are AI-constructed

Tags are **not** created by humans directly. The AI agent:
1. Analyzes the memory content (who, where, what, when)
2. Extracts candidate concepts
3. Searches existing tags for matches before creating new ones
4. Associates relevant tags with the memory

The CLI's `next_steps` field guides the AI on when and how to tag:
```json
"next_steps": [
  "gm tag <node-id> <tag-name>  — consider tagging with relevant people, places, or themes"
]
```

### Tags vs edges

| | Tags | Edges |
|---|---|---|
| Express | "Same topic/person/place" (symmetric) | "Caused by / happened after" (directed, typed) |
| Creation cost | Low (AI extracts from content) | High (infer type + direction + both endpoints) |
| Scaling | O(N) | O(N²) |

Complementary, not competing. Tags enable cheap discovery. Edges enable precise structural reasoning.

### Tag lifecycle

1. **Extraction** — AI extracts candidate concepts from memory content
2. **Deduplication** — AI searches existing tags before creating new ones
3. **Association** — tags attached to memories (one memory → many tags)
4. **Enrichment** — tag descriptions evolve, becoming living context
5. **Maintenance** — merge synonyms, split overloaded tags, retire orphans (all via proposals)

### Design decisions

- **Flat, not hierarchical** — hierarchies impose a single classification axis; real memory has overlapping dimensions
- **Future: tag-to-tag edges** — planned support for tag relationships (parent/child, synonym, related)
- **Separate table, not JSON properties** — indexed queries, FTS5 integration, event-sourced, cross-memory JOINs
- **Junction table has no metadata** — event log provides full history
- **2-5 tags per memory** — fewer loses signal, more creates noise

### Limitations

| Limitation | Mitigation |
|---|---|
| No directionality | By design — tags discover, edges structure |
| Synonym proliferation | AI must search before creating; periodic merge via proposals |
| Stale tags | Query orphaned tags; propose cleanup |
| Over-tagging | Behavioral guideline: 2-5 tags per memory |
| No weighted relationships | Edges with properties handle strength |
| Depends on AI quality | FTS5 as fallback; humans correct via proposals |

---

## Event Sourcing

All mutations are recorded as immutable events. Current state (nodes, edges, tags) is derived by projection.

**Why:**
- Full audit trail of every change
- Reconstruct state at any point in time
- Memory evolution analysis — how understanding changed over time

**Inline projection** — event and projection update in the same SQLite transaction. No async consumers, no eventual consistency. Simple and guaranteed consistent.

**Rebuild capability** — if projections corrupt, replay all events to reconstruct. Disaster recovery path, not normal operation.

---

## Proposal-First Write Model

AI agents don't write directly to the graph. All changes are staged as proposals, committed after confirmation.

**Why:**
- Prevents bad AI analysis from polluting the memory graph
- Atomic — all operations in a proposal succeed or none do
- Re-validated at commit time (graph may have changed)
- Human stays in control of the truth

---

## Storage: SQLite

Single SQLite database file. No external server.

**Why SQLite:**
- Local-first, zero config — single file, no network
- OLTP fit — CRUD + event appending are transactional workloads
- Graph traversal — recursive CTEs handle relationship chains at memory scale
- JSON columns — json_extract() for flexible properties
- Pure Go driver — no CGO dependency

**Why not others:**

| Alternative | Rejected because |
|---|---|
| Neo4j, DGraph | Requires standalone server — violates local-first |
| PostgreSQL | Not zero-config single-machine |
| DuckDB | OLAP-oriented, not OLTP |
| Plain JSON files | No queries, no transactions |

DuckDB could be added later as a read-only analytics layer (it reads SQLite directly).
