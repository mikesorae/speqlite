# speqlite — AI Reference

`speqlite` is a local-first CLI for managing specifications as structured state in SQLite, inspired by Terraform's plan/apply workflow. Markdown is an editable projection; SQLite is the source of truth.

## Install

```bash
go install github.com/mikesorae/speqlite/cmd/speqlite@latest
```

## Core Workflow

```
import <file>  →  plan     →  apply   →  render
(parse prose)    (review)    (commit)    (output .md)
```

```bash
speqlite init                     # create .spec/ workspace
speqlite import scratch/draft.md  # parse prose → write state.plan.json
speqlite plan                     # review pending changes
speqlite apply                    # commit plan to state.sqlite
speqlite render --all             # regenerate specs/*.md
```

## Commands

| Command | Purpose | Key flags |
|---|---|---|
| `init` | Create workspace | `--force` |
| `import <file>...` | Parse → plan | `--dry-run`, `--kind`, `--status` |
| `plan` | Show pending plan | `--summary` |
| `apply` | Commit plan | `--auto-approve`, `--dry-run` |
| `render [ID...]` | Write specs/*.md | `--all`, `--format markdown\|text\|json` |
| `search "<query>"` | FTS5/BM25 full-text search | `--kind`, `--status`, `--limit`, `--show-body` |
| `deps <ID>` | Dependency tree | `--depth`, `--direction out\|in\|both`, `--relation` |
| `validate` | Check integrity | `--strict`, `--fix` |
| `state list` | List all specs | `--kind`, `--status`, `--sort` |
| `state show <ID>` | Full spec detail + event history | — |
| `state export` | JSON snapshot of full state | `--format json\|snapshot`, `--output` |
| `state transition <ID>` | Generate status-change plan entry | `--to <status>` |
| `export` | Formal language export (stub) | `--format alloy\|smtlib\|tla\|lean\|datalog` |
| `version` | Print version | — |

Global flags: `--workspace <path>`, `--format text|json|json-pretty`, `--quiet`, `--no-color`, `--verbose`

## Data Model

### SpecNode

```json
{
  "id":         "CMD-IMPORT",              // ^[A-Z][A-Z0-9]*(-[A-Z0-9]+)+$  immutable
  "title":      "Import command",
  "kind":       "command",
  "status":     "fixed",
  "version":    3,                         // monotone counter for conflict detection
  "body":       "Full prose or Markdown",
  "hash":       "sha256:...",             // SHA-256 of id+title+kind+status+body
  "created_at": "2024-01-10T09:00:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

### Kinds

`requirement` | `command` | `architecture` | `data-model` | `state` | `constraint` | `test` | `glossary` | `note`

ID prefix inference: `CMD-` → command, `FR-` → requirement, `ARCH-` → architecture, `STATE-` → state, `TEST-` → test, `CONSTR-` → constraint, `GLOSS-` → glossary

### Status Lifecycle

```
draft → review → fixed → implemented → verified → deprecated
```

- Any non-deprecated state → `deprecated` (always allowed)
- `deprecated` → `draft` (reactivate)
- No arbitrary skipping; each transition must follow the graph
- `fixed` is the minimum stable status for dependency targets

### Relation Types

`depends_on` | `implements` | `verifies` | `supersedes` | `related_to`

Declared in Markdown body: `**Depends on**: CMD-PLAN, STATE-SQLITE` or `- depends_on: CMD-PLAN`  
Also accepted in YAML front matter: `relations: {depends_on: [CMD-PLAN]}`

## File Structure

```
.spec/state.sqlite         # canonical state DB  — commit to git
.spec/state.snapshot.json  # metadata snapshot   — commit to git
.spec/state.plan.json      # pending plan        — gitignore (ephemeral)
specs/<ID>.md              # rendered projections — commit to git
scratch/                   # input drafts        — user-managed
```

Recommended `.gitignore` additions:
```
.spec/state.plan.json
.spec/state.sqlite-wal
.spec/state.sqlite-shm
```

## JSON Output Schemas

### `search --format json`

```json
{
  "query": "plan apply",
  "results": [
    {"id": "CMD-APPLY", "title": "Apply command", "kind": "command", "status": "fixed", "rank": -1.23}
  ]
}
```

### `state export` (snapshot — body text omitted)

```json
{
  "version": 1,
  "snapshot_hash": "sha256:...",
  "taken_at": "2024-01-15T10:30:00Z",
  "specs": [
    {"id": "CMD-IMPORT", "title": "Import command", "kind": "command", "status": "fixed", "version": 3, "hash": "sha256:...", "updated_at": "2024-01-15T10:30:00Z"}
  ],
  "relations": [
    {"from_id": "CMD-APPLY", "relation": "depends_on", "to_id": "CMD-PLAN"}
  ],
  "constraints": []
}
```

Body text is not in the snapshot — use `state show <ID>` or query SQLite directly for body.

### `import --format json` (plan summary)

```json
{
  "source_files": ["scratch/feature.md"],
  "plan_hash": "sha256:...",
  "ops": [
    {"op": "create", "id": "CMD-IMPORT", "kind": "command", "title": "Import command"},
    {"op": "update", "id": "FR-001",     "fields": ["title", "body"]},
    {"op": "delete", "id": "CMD-OLD",    "mode": "deprecate"}
  ]
}
```

### `deps --format json`

```json
{
  "root": "CMD-APPLY",
  "nodes": [{"id": "CMD-APPLY", "title": "Apply command", "depth": 0}, {"id": "CMD-PLAN", "depth": 1}],
  "edges": [{"from": "CMD-APPLY", "relation": "depends_on", "to": "CMD-PLAN"}]
}
```

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Success |
| 1 | General error (I/O, parse) |
| 2 | Validation error |
| 3 | No pending plan |
| 4 | Conflict detected (concurrent mutation) |
| 5 | Workspace not initialised |

## Validation Errors

`E_DUPLICATE_ID` · `E_BROKEN_RELATION` · `E_INVALID_TRANSITION` · `E_CYCLIC_DEP` · `E_DRAFT_DEPENDENCY` · `E_DEPRECATED_DEPENDENCY`

## Scripting Examples

```bash
# Non-interactive apply in CI
speqlite import docs/spec.md && speqlite apply --auto-approve

# Extract spec IDs from search results
speqlite search "authentication" --format json | jq -r '.results[].id'

# Export state for consumption by another tool
speqlite state export --format json > spec-state.json

# Check dependency graph before deprecating a spec
speqlite deps CMD-IMPORT --direction in --format json | jq '.edges'
```
