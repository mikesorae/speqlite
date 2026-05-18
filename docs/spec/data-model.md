# Data Model

Speclite's canonical state is an SQLite database located at `.spec/state.sqlite`. This page documents every table, column, constraint, index, and the FTS virtual table.

---

## Overview

The schema contains five persistent objects:

| Object | Type | Purpose |
|---|---|---|
| `specs` | Table | Core specification nodes |
| `relations` | Table | Typed edges between spec nodes |
| `constraints` | Table | Formal expressions attached to specs |
| `event_log` | Table | Immutable audit trail of all mutations |
| `specs_fts` | Virtual table (FTS5) | Full-text search index |

All timestamps are stored as ISO-8601 UTC strings (`YYYY-MM-DDTHH:MM:SSZ`).

---

## Table: `specs`

Stores every specification node. Each row is one specification unit.

```sql
CREATE TABLE specs (
  id          TEXT    PRIMARY KEY,         -- canonical identifier, e.g. CMD-IMPORT
  title       TEXT    NOT NULL,            -- human-readable short title
  kind        TEXT    NOT NULL,            -- node type (see Kind Enumeration)
  status      TEXT    NOT NULL             -- lifecycle status (see State Machine)
              DEFAULT 'draft'
              CHECK (status IN ('draft','review','fixed','implemented','verified','deprecated')),
  version     INTEGER NOT NULL DEFAULT 1,  -- monotonically increasing mutation counter
  body        TEXT    NOT NULL DEFAULT '', -- full prose / Markdown body
  hash        TEXT    NOT NULL,            -- SHA-256 of canonical serialisation
  created_at  TEXT    NOT NULL,            -- ISO-8601 UTC creation timestamp
  updated_at  TEXT    NOT NULL             -- ISO-8601 UTC last-mutation timestamp
);

CREATE INDEX idx_specs_kind   ON specs(kind);
CREATE INDEX idx_specs_status ON specs(status);
```

### Column Details

#### `id`

The canonical identifier for a specification node. IDs follow the pattern `{PREFIX}-{SLUG}` where:

- `PREFIX` is an uppercase type abbreviation (e.g. `CMD`, `FR`, `STATE`, `ARCH`)
- `SLUG` is an uppercase alphanumeric-plus-hyphen token (e.g. `IMPORT`, `APPLY-01`)

Examples: `CMD-IMPORT`, `FR-001`, `STATE-SQLITE`, `ARCH-NORMALIZER`

Rules:
- Must be globally unique within a workspace
- Must match the regex `^[A-Z][A-Z0-9]*(-[A-Z0-9]+)+$`
- Immutable once created (rename = deprecate old + create new)

#### `title`

Short human-readable label. Used as the primary display string and as the weighted FTS field. Must not be empty.

#### `kind`

Classifies the node's semantic role. The normalizer uses `kind` to infer which rendering template and validation rules apply.

| Value | Meaning |
|---|---|
| `requirement` | Functional or non-functional requirement |
| `command` | CLI command specification |
| `architecture` | Architectural decision or component description |
| `data-model` | Schema, type, or data structure definition |
| `state` | A named state or lifecycle node |
| `constraint` | A formal or semi-formal constraint expression |
| `test` | A test case or acceptance criterion |
| `glossary` | Term definition |
| `note` | Informal note or context |

The set is extensible; unknown kinds are accepted and stored as-is.

#### `status`

Lifecycle state of the specification. See [State Machine](state-machine.md) for the full transition table.

| Value | Meaning |
|---|---|
| `draft` | Initial state; under authoring |
| `review` | Submitted for review; no further edits without state transition |
| `fixed` | Agreed upon; no further changes expected |
| `implemented` | Corresponding code/artefact exists |
| `verified` | Independently verified by test or audit |
| `deprecated` | Superseded or removed; retained for history |

#### `version`

Monotonically increasing integer. Starts at `1` on creation. Incremented by `1` on every `update_spec` event. Used for optimistic concurrency detection during plan/apply.

#### `body`

Full prose or Markdown content of the specification. May be empty for stub nodes. Stored verbatim; rendering is handled by the renderer package.

#### `hash`

SHA-256 hex digest of the canonical serialisation of the spec node. The canonical serialisation is:

```
{id}\n{title}\n{kind}\n{status}\n{body}
```

Used by the planner to detect content changes without full body comparison.

#### `created_at` / `updated_at`

ISO-8601 UTC timestamps. `created_at` is set once at row creation and never updated. `updated_at` is refreshed on every mutation.

---

## Table: `relations`

Stores directed, typed edges between spec nodes.

```sql
CREATE TABLE relations (
  from_id   TEXT NOT NULL REFERENCES specs(id) ON DELETE CASCADE,
  relation  TEXT NOT NULL
            CHECK (relation IN ('depends_on','implements','verifies','supersedes','related_to')),
  to_id     TEXT NOT NULL REFERENCES specs(id) ON DELETE RESTRICT,
  PRIMARY KEY (from_id, relation, to_id)
);

CREATE INDEX idx_relations_to_id ON relations(to_id);
```

### Relation Types

| Relation | Direction | Meaning |
|---|---|---|
| `depends_on` | `from` requires `to` | `from` cannot be implemented without `to` |
| `implements` | `from` provides `to` | `from` is the concrete realisation of `to` |
| `verifies` | `from` checks `to` | `from` is a test/audit for `to` |
| `supersedes` | `from` replaces `to` | `from` is the newer version that obsoletes `to` |
| `related_to` | bidirectional (by convention) | Loose thematic relationship |

### Constraints

- `ON DELETE CASCADE` on `from_id`: deleting a spec removes all its outgoing relations.
- `ON DELETE RESTRICT` on `to_id`: a spec cannot be deleted if other specs depend on it (prevents broken relations). To remove such a spec, first remove or reroute the incoming relations.
- The composite primary key prevents duplicate edges of the same type.
- Self-referencing edges (`from_id = to_id`) are forbidden at the application layer.

---

## Table: `constraints`

Stores formal or semi-formal constraint expressions attached to spec nodes. Designed for future export to formal verification tools.

```sql
CREATE TABLE constraints (
  id          TEXT PRIMARY KEY,           -- unique constraint identifier
  target_id   TEXT NOT NULL               -- spec node this constraint applies to
              REFERENCES specs(id) ON DELETE CASCADE,
  language    TEXT NOT NULL,             -- expression language (see below)
  expression  TEXT NOT NULL,             -- the constraint expression
  created_at  TEXT NOT NULL              -- ISO-8601 UTC
);

CREATE INDEX idx_constraints_target ON constraints(target_id);
```

### Expression Languages

| Value | Toolchain | Notes |
|---|---|---|
| `natural` | None | Free-form English invariant; not machine-checked |
| `alloy` | Alloy Analyser | Relational logic |
| `smtlib` | Z3, CVC5 | SMT-LIB 2.x |
| `tla` | TLC model checker | TLA+ |
| `lean` | Lean 4 | Dependent type theory |
| `datalog` | Soufflé, µZ | Deductive rules |

All non-`natural` languages are stored verbatim for future export; the MVP does not execute them.

---

## Table: `event_log`

Append-only audit trail. No row is ever updated or deleted. Provides full history of all state mutations.

```sql
CREATE TABLE event_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type   TEXT    NOT NULL,          -- see Event Types below
  spec_id      TEXT,                      -- may be NULL for workspace-level events
  payload_json TEXT    NOT NULL           -- JSON blob, schema varies by event_type
               DEFAULT '{}',
  created_at   TEXT    NOT NULL           -- ISO-8601 UTC
);

CREATE INDEX idx_event_log_spec_id    ON event_log(spec_id);
CREATE INDEX idx_event_log_event_type ON event_log(event_type);
CREATE INDEX idx_event_log_created_at ON event_log(created_at);
```

### Event Types

| `event_type` | Trigger | `payload_json` fields |
|---|---|---|
| `create_spec` | New spec node added via apply | `{id, title, kind, status}` |
| `update_spec` | Existing spec body or title mutated | `{id, old_hash, new_hash, fields_changed:[...]}` |
| `change_status` | Spec lifecycle transition | `{id, from_status, to_status}` |
| `create_relation` | New edge added | `{from_id, relation, to_id}` |
| `remove_relation` | Edge removed | `{from_id, relation, to_id}` |
| `create_constraint` | Formal constraint added | `{id, target_id, language}` |
| `apply_plan` | A plan file was applied | `{plan_hash, ops_count}` |
| `init_workspace` | Workspace initialised | `{version}` |

---

## Virtual Table: `specs_fts`

Full-text search index powered by SQLite FTS5 with BM25 ranking.

```sql
CREATE VIRTUAL TABLE specs_fts USING fts5(
  id    UNINDEXED,   -- stored but not tokenised
  title,             -- weighted 3× via rank() bm25 boost
  body,              -- full body text
  content='specs',   -- shadow table: keeps FTS in sync with specs
  content_rowid='rowid'
);

-- Triggers to keep FTS in sync with specs
CREATE TRIGGER specs_ai AFTER INSERT ON specs BEGIN
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;

CREATE TRIGGER specs_ad AFTER DELETE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
END;

CREATE TRIGGER specs_au AFTER UPDATE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;
```

### Search Query

```sql
SELECT
  s.id,
  s.title,
  s.kind,
  s.status,
  bm25(specs_fts, 0, 3.0, 1.0) AS rank   -- title weighted 3×
FROM specs_fts
JOIN specs s ON s.id = specs_fts.id
WHERE specs_fts MATCH ?
  AND s.kind   = ?   -- optional kind filter
  AND s.status = ?   -- optional status filter
ORDER BY rank
LIMIT ?;
```

The `bm25()` auxiliary function's column weights are:
- Column 0 (`id`): `0` (UNINDEXED, not ranked)
- Column 1 (`title`): `3.0` (title match boosted)
- Column 2 (`body`): `1.0` (baseline)

---

## Entity-Relationship Diagram

```
┌───────────┐      ┌───────────────┐      ┌───────────┐
│   specs   │◄─────│   relations   │─────►│   specs   │
│           │      │  from_id      │      │           │
│  id (PK)  │      │  relation     │      │  id (PK)  │
│  title    │      │  to_id        │      │  ...      │
│  kind     │      └───────────────┘      └───────────┘
│  status   │
│  version  │      ┌───────────────┐
│  body     │◄─────│  constraints  │
│  hash     │      │  id (PK)      │
│  ...      │      │  target_id    │
└───────────┘      │  language     │
      │            │  expression   │
      │            └───────────────┘
      │
      ▼
┌───────────────┐      ┌─────────────┐
│   event_log   │      │  specs_fts  │
│  id (PK auto) │      │  (virtual)  │
│  event_type   │      │  id         │
│  spec_id      │      │  title      │
│  payload_json │      │  body       │
│  created_at   │      └─────────────┘
└───────────────┘
```

---

## Schema Version & Migration

The schema version is stored in the SQLite `user_version` pragma:

```sql
PRAGMA user_version = 1;
```

Migrations are applied sequentially. Each migration file is named `NNN_description.sql`. The apply engine checks `user_version` at startup and runs pending migrations before any operation.

---

## Full DDL (Single Block)

```sql
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA user_version = 1;

CREATE TABLE specs (
  id          TEXT    PRIMARY KEY,
  title       TEXT    NOT NULL,
  kind        TEXT    NOT NULL,
  status      TEXT    NOT NULL DEFAULT 'draft'
              CHECK (status IN ('draft','review','fixed','implemented','verified','deprecated')),
  version     INTEGER NOT NULL DEFAULT 1,
  body        TEXT    NOT NULL DEFAULT '',
  hash        TEXT    NOT NULL,
  created_at  TEXT    NOT NULL,
  updated_at  TEXT    NOT NULL
);

CREATE INDEX idx_specs_kind   ON specs(kind);
CREATE INDEX idx_specs_status ON specs(status);

CREATE TABLE relations (
  from_id   TEXT NOT NULL REFERENCES specs(id) ON DELETE CASCADE,
  relation  TEXT NOT NULL
            CHECK (relation IN ('depends_on','implements','verifies','supersedes','related_to')),
  to_id     TEXT NOT NULL REFERENCES specs(id) ON DELETE RESTRICT,
  PRIMARY KEY (from_id, relation, to_id)
);

CREATE INDEX idx_relations_to_id ON relations(to_id);

CREATE TABLE constraints (
  id          TEXT PRIMARY KEY,
  target_id   TEXT NOT NULL REFERENCES specs(id) ON DELETE CASCADE,
  language    TEXT NOT NULL,
  expression  TEXT NOT NULL,
  created_at  TEXT NOT NULL
);

CREATE INDEX idx_constraints_target ON constraints(target_id);

CREATE TABLE event_log (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  event_type   TEXT    NOT NULL,
  spec_id      TEXT,
  payload_json TEXT    NOT NULL DEFAULT '{}',
  created_at   TEXT    NOT NULL
);

CREATE INDEX idx_event_log_spec_id    ON event_log(spec_id);
CREATE INDEX idx_event_log_event_type ON event_log(event_type);
CREATE INDEX idx_event_log_created_at ON event_log(created_at);

CREATE VIRTUAL TABLE specs_fts USING fts5(
  id    UNINDEXED,
  title,
  body,
  content='specs',
  content_rowid='rowid'
);

CREATE TRIGGER specs_ai AFTER INSERT ON specs BEGIN
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;

CREATE TRIGGER specs_ad AFTER DELETE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
END;

CREATE TRIGGER specs_au AFTER UPDATE ON specs BEGIN
  INSERT INTO specs_fts(specs_fts, rowid, id, title, body)
  VALUES ('delete', old.rowid, old.id, old.title, old.body);
  INSERT INTO specs_fts(rowid, id, title, body)
  VALUES (new.rowid, new.id, new.title, new.body);
END;
```
