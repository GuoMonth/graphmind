# Naming Conventions

The `gm` CLI is consumed by AI agents. Every word is a token in a structured language. Naming consistency directly determines AI agent efficiency and correctness.

**One concept = one word, everywhere.** CLI flags, Go code, DB schema, JSON fields, docs — all use the same vocabulary. No synonyms. No abbreviations. No context-dependent meaning.

---

## Canonical Lexicon

### Entities (nouns)

These are the only allowed nouns for domain objects:

| Canonical word | Meaning | Scope |
|---|---|---|
| `node` | A vertex in the project graph | CLI resource, DB table, Go type |
| `edge` | A directed relationship between two nodes | CLI resource, DB table, Go type |
| `tag` | An AI-extracted semantic label | CLI resource, DB table, Go type |
| `proposal` | A staged batch of changes awaiting confirmation | CLI resource, DB table, Go type |
| `event` | An immutable record of a past mutation | CLI resource, DB table, Go type |
| `graph` | The overall structure of nodes and edges | CLI resource (virtual, not a DB table) |

### Actions (verbs)

These are the only allowed verbs for operations:

| Canonical verb | Meaning | Used in |
|---|---|---|
| `create` | Make a new entity | CLI action, Go method, event type |
| `get` | Retrieve exactly one entity by ID | CLI action, Go method |
| `list` | Retrieve multiple entities with optional filters | CLI action, Go method |
| `update` | Modify an existing entity (partial update) | CLI action, Go method, event type |
| `delete` | Remove an entity permanently | CLI action, Go method, event type |
| `search` | Find entities by full-text keyword (FTS5) | CLI action, Go method |
| `query` | Find nodes using multi-modal criteria (keyword + tag + filter + expand) | CLI action, Go method |
| `traverse` | Walk the graph from a starting node following edges | CLI action, Go method |
| `commit` | Apply a pending proposal to the graph | CLI action, Go method, event type |
| `reject` | Discard a pending proposal | CLI action, Go method, event type |
| `tag` | Associate a tag with a node | CLI action (on `node` resource) |
| `untag` | Remove a tag association from a node | CLI action (on `node` resource) |

### Properties (field names)

| Canonical word | Meaning | Where used |
|---|---|---|
| `type` | The category of a node or edge | CLI flag, DB column, JSON field, Go field |
| `status` | The current lifecycle state | CLI flag, DB column, JSON field, Go field |
| `title` | The human-readable name of a node | DB column, JSON field, Go field |
| `description` | Additional context text | DB column, JSON field, Go field |
| `properties` | Flexible key-value attributes (JSON object) | DB column, JSON field, Go field |
| `payload` | Structured data in events and proposals (JSON) | DB column, JSON field, Go field |
| `direction` | Traversal direction: `outgoing`, `incoming`, `both` | CLI flag, Go field |

### References (pointers to entities)

| Canonical word | Meaning | Where used |
|---|---|---|
| `id` | The primary key (UUID v7) of the current entity | CLI flag `--id`, DB PK, JSON, Go |
| `from_id` | Source node UUID of an edge | DB column, JSON field, CLI flag `--from-id` |
| `to_id` | Target node UUID of an edge | DB column, JSON field, CLI flag `--to-id` |
| `node_id` | A reference to a node by UUID | DB column, JSON field, CLI flag `--node-id` |
| `tag_id` | A reference to a tag by UUID | DB column, JSON field, CLI flag `--tag-id` |
| `tag_name` | A reference to a tag by its unique name | CLI flag `--tag-name` |
| `from_reference` | Index of a node-creating operation within the same proposal | JSON field in proposal operations |
| `to_reference` | Index of a node-creating operation within the same proposal | JSON field in proposal operations |

---

## Banned Synonyms

When writing CLI commands, Go code, DB schemas, JSON fields, or documentation — **never** use the left column. Always use the right column.

| ❌ Never use | ✅ Always use | Why |
|---|---|---|
| item, entity, vertex, point | `node` | One word for graph vertices |
| link, connection, relation, relationship, arc | `edge` | One word for graph relationships |
| label, category, topic, keyword (as noun) | `tag` | One word for semantic labels |
| draft, changeset, batch | `proposal` | One word for staged changes |
| add, new, make, insert | `create` | One verb for creation |
| remove, destroy, drop, purge | `delete` | One verb for deletion |
| fetch, retrieve, read, show, find (single) | `get` | One verb for single-entity retrieval |
| find, enumerate, all, query (for listing) | `list` | One verb for multi-entity retrieval |
| modify, change, edit, patch, set | `update` | One verb for mutation |
| attributes, metadata, fields, data, extras | `properties` | One word for flexible attributes |
| body, content, data (for events) | `payload` | One word for event/proposal data |
| source_id, origin_id, parent_id | `from_id` | One word for edge source |
| target_id, destination_id, child_id | `to_id` | One word for edge target |
| state, phase, stage | `status` | One word for lifecycle state |
| kind, category, class, sort | `type` | One word for entity category |
| ref, idx | `reference`, `index` | No abbreviations for domain concepts |

---

## Flag Suffix Rules

Every CLI flag that accepts a value must signal **what kind of value** through its suffix.

| Suffix | Meaning | Examples |
|---|---|---|
| `-id` | UUID v7 primary key | `--id`, `--from-id`, `--to-id`, `--node-id`, `--tag-id` |
| `-name` | Unique name string | `--tag-name` |
| `-type` | Type enum value | `--type`, `--edge-type` |
| (none) | Self-evident from flag name | `--status`, `--direction`, `--keyword`, `--limit`, `--after` |

**Hard rule**: A flag without `-id` suffix MUST NOT accept a UUID. A flag with `-id` suffix MUST accept a UUID.

---

## Direction Values

Traversal and edge query direction uses full words:

| ✅ Use | ❌ Not |
|---|---|
| `outgoing` | `out` |
| `incoming` | `in` |
| `both` | `all`, `any` |

---

## CLI Naming Patterns

### Command structure

```
gm <resource> <action> [--flag-name value]
```

- Resource: always singular noun (`node`, `edge`, `tag`, `proposal`, `event`, `graph`)
- Action: always a verb from the canonical list
- Flags: lowercase, hyphen-separated (`--from-id`, `--tag-name`, `--max-depth`)

### Flag naming

| Pattern | Example | Rule |
|---|---|---|
| Entity self-reference | `--id <uuid>` | The entity being acted upon |
| Entity cross-reference | `--from-id`, `--to-id`, `--node-id`, `--tag-id` | Reference to another entity |
| Name reference | `--tag-name "payment"` | Reference by unique name, not ID |
| Type filter | `--type task`, `--edge-type depends_on` | Filter by type enum |
| Status filter | `--status open` | Filter by status enum |
| Text search | `--keyword "payment"` | FTS5 search term |
| Limit/cursor | `--limit 20`, `--after <cursor>` | Pagination |
| Graph expansion | `--max-depth 3`, `--expand 2` | Depth control |
| Direction | `--direction outgoing` | Traversal direction |

---

## Go Naming Patterns

### Type names

Pattern: `{Entity}` for domain types, `{Action}{Entity}{Suffix}` for I/O types.

| Category | Pattern | Examples |
|---|---|---|
| Domain types | `{Entity}` | `Node`, `Edge`, `Tag`, `Proposal`, `Event` |
| Create input | `Create{Entity}Input` | `CreateNodeInput`, `CreateEdgeInput`, `CreateTagInput` |
| Update input | `Update{Entity}Input` | `UpdateNodeInput`, `UpdateTagInput` |
| List filter | `{Entity}Filter` | `NodeFilter`, `EdgeFilter`, `TagFilter`, `EventFilter` |
| Graph operations | `Graph{Action}Input` | `GraphQueryInput`, `GraphTraverseInput` |
| Results | `Graph{Action}Result` | `GraphQueryResult`, `GraphTraverseResult` |
| Proposal result | `ProposalCommitResult` | Specific to proposal commit |
| Statistics | `GraphStats` | Graph-level statistics |

### Method names

On multi-entity services (e.g., `GraphService`), methods include the entity name:

```go
type GraphService interface {
    CreateNode(...)
    GetNode(...)
    ListNodes(...)     // plural for list
    UpdateNode(...)
    DeleteNode(...)
    CreateEdge(...)
    GetEdge(...)
    ListEdges(...)     // plural for list
    DeleteEdge(...)
    TagNode(...)
    UntagNode(...)
    Query(...)         // graph-level, no entity suffix needed
    Traverse(...)      // graph-level
    Stats(...)         // graph-level
}
```

On single-entity services, methods use bare verbs:

```go
type TagService interface {
    Create(...)
    Get(...)
    List(...)
    Update(...)
    Delete(...)
    Search(...)
}
```

### Field names

| Go field | JSON field | DB column | CLI flag |
|---|---|---|---|
| `ID` | `id` | `id` | `--id` |
| `Type` | `type` | `type` | `--type` |
| `Status` | `status` | `status` | `--status` |
| `Title` | `title` | `title` | — |
| `Description` | `description` | `description` | — |
| `Properties` | `properties` | `properties` | — |
| `FromID` | `from_id` | `from_id` | `--from-id` |
| `ToID` | `to_id` | `to_id` | `--to-id` |
| `NodeID` | `node_id` | `node_id` | `--node-id` |
| `TagID` | `tag_id` | `tag_id` | `--tag-id` |
| `CreatedAt` | `created_at` | `created_at` | — |
| `UpdatedAt` | `updated_at` | `updated_at` | — |
| `CommittedAt` | `committed_at` | `committed_at` | — |
| `FromReference` | `from_reference` | — | — |
| `ToReference` | `to_reference` | — | — |
| `MaxDepth` | `max_depth` | — | `--max-depth` |
| `Direction` | `direction` | — | `--direction` |

### Acronym casing in Go

Go convention: acronyms are all-caps.

| ✅ Correct | ❌ Wrong |
|---|---|
| `ID` | `Id` |
| `UUID` | `Uuid` |
| `NodeID` | `NodeId` |
| `FromID` | `FromId` |
| `TagID` | `TagId` |
| `FTS` | `Fts` |
| `JSON` | `Json` |
| `SQL` | `Sql` |
| `HTTP` | `Http` |
| `URL` | `Url` |

### Variable and parameter names

| Context | Convention | Example |
|---|---|---|
| Entity ID parameter | `{entity}ID` | `nodeID`, `tagID`, `edgeID`, `proposalID` |
| Current entity ID | `id` | `func (s *Store) Get(ctx context.Context, id string)` |
| Input parameter | `input` | `func (s *Store) Create(ctx context.Context, input CreateNodeInput)` |
| Filter parameter | `filter` | `func (s *Store) List(ctx context.Context, filter NodeFilter)` |
| Context parameter | `ctx` | Always first parameter (Go convention) |
| Database handle | `db` | `*sql.DB` instance |
| Transaction | `tx` | `*sql.Tx` instance |
| Error | `err` | Standard Go |
| Slice of entities | `nodes`, `edges`, `tags` | Plural of entity name |
| Single entity | `node`, `edge`, `tag` | Singular of entity name |

---

## Event Type Naming

Pattern: `{entity}_{past_tense_verb}`

| Event type | Entity | Verb |
|---|---|---|
| `node_created` | node | created |
| `node_updated` | node | updated |
| `node_deleted` | node | deleted |
| `node_tagged` | node | tagged |
| `node_untagged` | node | untagged |
| `edge_created` | edge | created |
| `edge_deleted` | edge | deleted |
| `tag_created` | tag | created |
| `tag_updated` | tag | updated |
| `tag_deleted` | tag | deleted |
| `proposal_created` | proposal | created |
| `proposal_committed` | proposal | committed |
| `proposal_rejected` | proposal | rejected |

---

## Proposal Operation Naming

Pattern: `{verb}_{entity}`

| Operation action | Meaning |
|---|---|
| `create_node` | Create a new node |
| `update_node` | Update an existing node |
| `delete_node` | Delete an existing node |
| `create_edge` | Create a new edge |
| `delete_edge` | Delete an existing edge |
| `create_tag` | Create a new tag |
| `tag_node` | Associate a tag with a node |
| `untag_node` | Remove a tag from a node |

---

## Cross-Layer Consistency Matrix

The same concept must use the same root word across all layers:

| Concept | CLI flag | JSON field | DB column | Go field | Go param |
|---|---|---|---|---|---|
| Node's UUID | `--id` | `id` | `id` | `ID` | `id` |
| Edge source | `--from-id` | `from_id` | `from_id` | `FromID` | `fromID` |
| Edge target | `--to-id` | `to_id` | `to_id` | `ToID` | `toID` |
| Node reference | `--node-id` | `node_id` | `node_id` | `NodeID` | `nodeID` |
| Tag reference | `--tag-id` | `tag_id` | `tag_id` | `TagID` | `tagID` |
| Tag by name | `--tag-name` | `tag_name` | `name` | `TagName` | `tagName` |
| Entity type | `--type` | `type` | `type` | `Type` | — |
| Entity status | `--status` | `status` | `status` | `Status` | — |
| Search keyword | `--keyword` | — | — | `Keyword` | `keyword` |
| Max traversal depth | `--max-depth` | `max_depth` | — | `MaxDepth` | `maxDepth` |
| Traversal direction | `--direction` | `direction` | — | `Direction` | `direction` |
| Neighborhood expand | `--expand` | `expand` | — | `Expand` | `expand` |

---

## No-Abbreviation Rule

Use complete English words for all identifiers visible to AI agents. Abbreviations create ambiguity.

**Allowed abbreviations** (universally understood, shorter than 6 characters saved):

| Abbreviation | Full word | Why allowed |
|---|---|---|
| `id` | identifier | Universal, saves 8 chars |
| `ctx` | context | Go convention, ubiquitous |
| `db` | database | Universal, saves 6 chars |
| `tx` | transaction | Go convention for `*sql.Tx` |
| `err` | error | Go convention, ubiquitous |
| `ok` | okay | JSON envelope convention |
| `fts` | full-text search | Only in internal table names (`nodes_fts`) |

**Everything else must be spelled out:**

| ❌ Abbreviated | ✅ Full word |
|---|---|
| `ref` | `reference` |
| `desc` | `description` |
| `prop`, `props` | `properties` |
| `cfg`, `conf` | `config` |
| `msg` | `message` |
| `num` | `count` (preferred) or `number` |
| `idx` | `index` |
| `val` | `value` |
| `param`, `params` | `parameter`, `parameters` |
| `info` | (use a specific name instead) |
| `tmp`, `temp` | `temporary` (or avoid entirely) |
| `max` | (allowed as prefix: `max_depth`, `MaxDepth`) |
| `min` | (allowed as prefix: `min_complexity`) |
