# CLI Reference

Complete reference for all `speqlite` commands, flags, exit codes, and output formats.

---

## Global Flags

These flags are available on every command.

| Flag | Type | Default | Description |
|---|---|---|---|
| `--workspace` | string | `.` | Path to the workspace root (must contain `.spec/`) |
| `--format` | string | `text` | Output format: `text`, `json`, `json-pretty` |
| `--quiet` | bool | `false` | Suppress informational output; only errors go to stderr |
| `--no-color` | bool | `false` | Disable ANSI colour codes |
| `--verbose` | bool | `false` | Increase log verbosity |
| `--version` | — | — | Print version and exit |
| `--help`, `-h` | — | — | Print command help and exit |

---

## Exit Codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error (I/O, parsing, unexpected) |
| `2` | Validation error (structural or constraint violation) |
| `3` | No pending plan (used by `apply` when there is nothing to apply) |
| `4` | Conflict detected (apply would overwrite a concurrent change) |
| `5` | Workspace not initialised (`.spec/` directory missing) |

---

## `speqlite init`

Initialise a new Speclite workspace in the current directory.

### Synopsis

```
speqlite init [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--force` | bool | `false` | Re-initialise an existing workspace (non-destructive) |

### Behaviour

1. Creates `.spec/` directory.
2. Creates `.spec/state.sqlite` with the full DDL schema.
3. Writes `PRAGMA user_version = 1` to `state.sqlite`.
4. Creates empty `specs/`, `scratch/`, and `changes/` directories.
5. Writes a `change_status`-equivalent `init_workspace` event to `event_log`.
6. Prints confirmation to stdout.

### Output (text)

```
Initialised Speclite workspace at /path/to/project
  .spec/state.sqlite   created
  specs/               created
  scratch/             created
  changes/             created
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | Success |
| `1` | Directory not writable |
| `5` | Already initialised and `--force` not set |

---

## `speqlite import`

Parse one or more input files and compute a pending plan. Does **not** mutate state.

### Synopsis

```
speqlite import <file> [<file>...] [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--dry-run` | bool | `false` | Compute plan but do not write `state.plan.json` |
| `--id-prefix` | string | `""` | Override the ID prefix used when generating new IDs |
| `--kind` | string | `""` | Force all parsed nodes to this kind (overrides inference) |
| `--status` | string | `draft` | Initial status for newly created specs |

### Behaviour

1. Reads each file and dispatches to the appropriate parser (Markdown or plain text).
2. Runs the normalizer to produce a list of candidate `SpecNode` values.
3. Compares candidate nodes against the current state snapshot.
4. Produces a plan (list of `create`, `update`, `delete` operations).
5. Writes the plan to `.spec/state.plan.json` (unless `--dry-run`).
6. Prints a summary of the plan.

### Output (text)

```
Parsed scratch/my-feature.md → 3 spec nodes

Plan:
  + create CMD-IMPORT        [command]  "Import command"
  + create CMD-APPLY         [command]  "Apply command"
  ~ update FR-001            [requirement]  "Functional Requirement 1"

3 changes pending. Run `speqlite apply` to commit.
```

### Output (json)

```json
{
  "source_files": ["scratch/my-feature.md"],
  "plan_hash": "sha256:abc123...",
  "ops": [
    {"op": "create", "id": "CMD-IMPORT", "kind": "command", "title": "Import command"},
    {"op": "create", "id": "CMD-APPLY",  "kind": "command", "title": "Apply command"},
    {"op": "update", "id": "FR-001",     "fields": ["title", "body"]}
  ]
}
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | Plan generated successfully |
| `1` | File not found or unreadable |
| `2` | Validation error in parsed content |
| `5` | Workspace not initialised |

---

## `speqlite plan`

Display the current pending plan from `.spec/state.plan.json`.

### Synopsis

```
speqlite plan [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--summary` | bool | `false` | Print only the count per operation type |

### Output (text)

```
Pending plan (3 operations):

  + CMD-IMPORT   [command]     create   "Import command"
  ~ CMD-RENDER   [command]     update   title changed
  - CMD-OLD      [command]     delete   deprecated
```

Legend: `+` = create, `~` = update, `-` = delete/deprecate.

### Output (json)

Same structure as the `state.plan.json` file. See [Plan & Apply](plan-apply.md).

### Exit Codes

| Code | Condition |
|---|---|
| `0` | Plan shown |
| `3` | No pending plan (`state.plan.json` absent or empty) |
| `5` | Workspace not initialised |

---

## `speqlite apply`

Apply the pending plan to `.spec/state.sqlite`. This is the only command that mutates canonical state.

### Synopsis

```
speqlite apply [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--auto-approve` | bool | `false` | Skip interactive confirmation prompt |
| `--dry-run` | bool | `false` | Show what would be applied; do not mutate state |

### Behaviour

1. Reads `.spec/state.plan.json`.
2. Validates the plan (schema check, conflict detection, transition validity).
3. (If interactive) Prompts for confirmation.
4. Executes all operations in a single SQLite transaction.
5. Appends events to `event_log` for every operation.
6. Updates `state.snapshot.json` to reflect new state.
7. Removes `state.plan.json`.

### Output (text)

```
Applying 3 operations...

  ✓ create CMD-IMPORT
  ✓ create CMD-APPLY
  ✓ update CMD-RENDER

Applied successfully. State updated.
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | All operations applied |
| `2` | Validation error; apply aborted |
| `3` | No pending plan |
| `4` | Conflict: version mismatch detected |
| `5` | Workspace not initialised |

---

## `speqlite render`

Generate Markdown projections from state.

### Synopsis

```
speqlite render [<spec-id>...] [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--all` | bool | `false` | Render all specs |
| `--output` | string | `specs/` | Output directory for rendered files |
| `--format` | string | `markdown` | Output format: `markdown`, `text`, `json` |
| `--status` | string | `""` | Only render specs with this status |
| `--kind` | string | `""` | Only render specs of this kind |

### Output

For each rendered spec, a file is created at `{output}/{id}.md`:

```markdown
# CMD-IMPORT

**Kind**: command
**Status**: fixed
**Version**: 3
**Updated**: 2024-01-15T10:30:00Z

## Description

The import command reads Markdown or plain-text files...

## Relations

- depends_on: [STATE-SQLITE](STATE-SQLITE.md)
- implements: [FR-001](FR-001.md)

---
<!-- speqlite:id=CMD-IMPORT speqlite:version=3 speqlite:hash=abc123 -->
```

The HTML comment at the end is a *roundtrip marker* used by the importer to detect previously-rendered specs.

### Exit Codes

| Code | Condition |
|---|---|
| `0` | All requested specs rendered |
| `1` | Output directory not writable |
| `5` | Workspace not initialised |

---

## `speqlite search`

Full-text search over spec titles and bodies using FTS5/BM25.

### Synopsis

```
speqlite search <query> [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--limit` | int | `20` | Maximum number of results |
| `--kind` | string | `""` | Filter by spec kind |
| `--status` | string | `""` | Filter by spec status |
| `--show-body` | bool | `false` | Include body excerpt in output |

### Output (text)

```
CMD-APPLY      [command]     fixed       "Apply command"
CMD-PLAN       [command]     fixed       "Plan command"
FR-005         [requirement] implemented "Plan/apply workflow requirement"
```

### Output (json)

```json
{
  "query": "plan apply",
  "results": [
    {"id": "CMD-APPLY", "title": "Apply command", "kind": "command", "status": "fixed", "rank": -1.23},
    {"id": "CMD-PLAN",  "title": "Plan command",  "kind": "command", "status": "fixed", "rank": -1.05}
  ]
}
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | Search completed (zero results is still exit 0) |
| `1` | FTS table not available |
| `5` | Workspace not initialised |

---

## `speqlite deps`

Display the dependency graph for a spec node.

### Synopsis

```
speqlite deps <spec-id> [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--depth` | int | `0` | Max traversal depth (0 = unlimited) |
| `--direction` | string | `out` | Edge direction: `out` (what it depends on), `in` (what depends on it), `both` |
| `--relation` | string | `""` | Filter by relation type |

### Output (text)

```
CMD-APPLY
 ├── depends_on → CMD-PLAN
 │    └── depends_on → STATE-SQLITE
 └── depends_on → STATE-SQLITE
```

### Output (json)

```json
{
  "root": "CMD-APPLY",
  "nodes": [
    {"id": "CMD-APPLY", "title": "Apply command", "depth": 0},
    {"id": "CMD-PLAN",  "title": "Plan command",  "depth": 1},
    {"id": "STATE-SQLITE", "title": "SQLite state", "depth": 1}
  ],
  "edges": [
    {"from": "CMD-APPLY", "relation": "depends_on", "to": "CMD-PLAN"},
    {"from": "CMD-APPLY", "relation": "depends_on", "to": "STATE-SQLITE"},
    {"from": "CMD-PLAN",  "relation": "depends_on", "to": "STATE-SQLITE"}
  ]
}
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | Graph displayed |
| `1` | Spec ID not found |
| `2` | Cyclic dependency detected |
| `5` | Workspace not initialised |

---

## `speqlite validate`

Run structural and constraint validation against the current state.

### Synopsis

```
speqlite validate [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--fix` | bool | `false` | Auto-fix trivial issues (e.g. normalise IDs) |
| `--strict` | bool | `false` | Treat warnings as errors |

### Checks Performed

| Check | Error Code | Severity |
|---|---|---|
| Duplicate IDs | `E_DUPLICATE_ID` | Error |
| Broken relation (target missing) | `E_BROKEN_RELATION` | Error |
| Invalid status transition in plan | `E_INVALID_TRANSITION` | Error |
| Cyclic dependency | `E_CYCLIC_DEP` | Error |
| `draft` target in stable dependency | `E_DRAFT_DEPENDENCY` | Warning |
| `deprecated` target in live dependency | `E_DEPRECATED_DEPENDENCY` | Warning |
| Spec with no title | `E_MISSING_TITLE` | Warning |
| Spec with no body | `E_EMPTY_BODY` | Warning (if `--strict`) |

### Output (text)

```
Validation complete.

Errors (2):
  E_BROKEN_RELATION  CMD-APPLY depends_on MISSING-NODE (not found)
  E_CYCLIC_DEP       CMD-A → CMD-B → CMD-A

Warnings (1):
  E_DRAFT_DEPENDENCY  CMD-RENDER depends_on FR-002 (draft)

Exit code: 2
```

### Exit Codes

| Code | Condition |
|---|---|
| `0` | No errors (warnings may exist) |
| `2` | One or more errors found |
| `5` | Workspace not initialised |

---

## `speqlite state`

Subcommand group for inspecting canonical state.

### `speqlite state list`

```
speqlite state list [flags]
```

| Flag | Description |
|---|---|
| `--status <s>` | Filter by status |
| `--kind <k>` | Filter by kind |
| `--sort <field>` | Sort by: `id`, `title`, `updated_at`, `status` (default: `id`) |

Output:

```
ID             KIND          STATUS        TITLE
CMD-APPLY      command       fixed         Apply command
CMD-IMPORT     command       fixed         Import command
CMD-PLAN       command       fixed         Plan command
FR-001         requirement   implemented   Import functional requirement
STATE-SQLITE   state         verified      SQLite canonical state
```

### `speqlite state show`

```
speqlite state show <spec-id> [flags]
```

Prints full details of a single spec including body, relations, and constraints.

Output:

```
ID:         CMD-APPLY
Title:      Apply command
Kind:       command
Status:     fixed
Version:    3
Hash:       sha256:abc123...
Created:    2024-01-10T09:00:00Z
Updated:    2024-01-15T10:30:00Z

Body:
  The apply command commits the pending plan to state.sqlite...

Relations:
  depends_on → CMD-PLAN
  depends_on → STATE-SQLITE

Constraints: (none)
```

### `speqlite state export`

```
speqlite state export [flags]
```

| Flag | Description |
|---|---|
| `--format <f>` | Export format: `json`, `snapshot` (default: `json`) |
| `--output <path>` | Output file (default: stdout) |

Exports the complete state as JSON. Equivalent to the content of `state.snapshot.json`.

### `speqlite state transition`

```
speqlite state transition <spec-id> --to <status> [flags]
```

| Flag | Description |
|---|---|
| `--to <status>` | Target status (required) |

Generates a plan entry for the status transition. Run `apply` to commit.

---

## `speqlite export`

Export specs to formal verification languages.

### Synopsis

```
speqlite export [flags]
```

### Flags

| Flag | Type | Default | Description |
|---|---|---|---|
| `--format` | string | — | Required. One of: `alloy`, `smtlib`, `tla`, `lean`, `datalog` |
| `--output` | string | stdout | Output file path |
| `--status` | string | `verified` | Only export specs with this status |
| `--include-all` | bool | `false` | Export all statuses |

!!! note
    Formal export stubs are included in the MVP binary but produce skeleton output only. Full expression generation is a post-MVP feature.

---

## `speqlite version`

Print version information.

```
speqlite version
```

Output:

```
speqlite v0.1.0 (commit: abc1234, built: 2024-01-15)
```
