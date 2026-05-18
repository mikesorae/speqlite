# Speclite

A local-first specification state management CLI, inspired by Terraform's plan/apply workflow.

Canonical state lives in SQLite. Markdown is a regeneratable projection for human editing.

```
Markdown / Plain Text
  ↓ import
Normalized Spec State
  ↓ plan
Diff / Pending Changes
  ↓ apply
SQLite State
  ↓ render
Markdown / Plain Text
```

## Quick Start

```bash
# 1. Initialise a workspace
speclite init

# 2. Write a rough spec in Markdown
cat > scratch/my-feature.md << 'EOF'
# CMD-IMPORT

Import command parses Markdown files into the spec state.

## CMD-APPLY

Apply command mutates SQLite state from the current plan.

depends_on CMD-IMPORT
EOF

# 3. Parse the file and generate a plan (no state mutation)
speclite import scratch/my-feature.md

# 4. Review the plan
speclite plan

# 5. Apply pending changes to SQLite
speclite apply

# 6. Regenerate Markdown from canonical state
speclite render --all

# 7. Search specs by full-text query
speclite search "import apply"

# 8. Inspect the dependency graph
speclite deps CMD-APPLY

# 9. Validate structural integrity
speclite validate

# 10. List all specs in state
speclite state list
```

## Installation

### From source

```bash
git clone <repo>
cd speclite
make install      # installs to $GOPATH/bin/speclite
```

### Build locally

```bash
make build        # outputs bin/speclite
./bin/speclite --help
```

## Commands

| Command | Description |
|---|---|
| `speclite init` | Initialise a new workspace (creates `.spec/`, `specs/`, etc.) |
| `speclite import <file>` | Parse Markdown/plain-text into a plan (no state mutation) |
| `speclite plan` | Display the pending plan from `.spec/state.plan.json` |
| `speclite apply` | Apply the pending plan to SQLite state |
| `speclite render [--all] [--format] [ID]` | Render specs from state to files in `specs/` |
| `speclite search "<query>"` | Full-text search with FTS5 BM25 ranking |
| `speclite deps <ID>` | Print the dependency tree for a spec |
| `speclite validate` | Structural validation (cycles, dangling refs, missing fields) |
| `speclite state list` | List all specs in state |
| `speclite state show <ID>` | Show full spec details and event history |
| `speclite state export` | Export full state snapshot as JSON |

### render flags

```
--all             Render all non-deprecated specs
--format          Output format: markdown (default), text, json
```

### search flags

```
--type <kind>     Filter by kind: command, requirement, state, constraint
--status <status> Filter by status: draft, review, fixed, implemented, verified, deprecated
```

### state list flags

```
--type <kind>     Filter by kind
--status <status> Filter by status
```

## File Structure

After `speclite init`, the workspace looks like:

```
.spec/
  state.sqlite          # Canonical SQLite state (source of truth)
  state.plan.json       # Pending changes (created by import, consumed by apply)
  state.snapshot.json   # Last-known-good state snapshot (created by apply)
specs/                  # Rendered Markdown/text/JSON output (regeneratable)
scratch/                # Working area for draft Markdown files
changes/                # Optional: track change notes
```

## Spec Lifecycle

```
draft → review → fixed → implemented → verified → deprecated
```

Any status may transition to `deprecated`. Cyclic dependency detection runs on `depends_on` edges.

## Supported Relation Types

- `depends_on` — A requires B
- `implements` — A implements B
- `verifies` — A verifies B
- `supersedes` — A replaces B
- `related_to` — A is related to B

Relations are extracted automatically from Markdown text during `import`.

## Development

```bash
make test           # run all tests
make test-verbose   # verbose test output
make test-cover     # generate coverage report
make lint           # golangci-lint (requires golangci-lint installed)
make clean          # remove build artifacts
```

## Design Philosophy

- **Markdown is not canonical** — it is an editable projection; canonical state is SQLite
- **Plan before apply** — all state mutations require an explicit `apply` step
- **Immutable event log** — every state change is recorded in `event_log`
- **AI-readable** — spec state is structured for programmatic consumption
- **Future-proof** — constraints table is designed for eventual formal verification export
