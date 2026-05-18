# speqlite

[![Go Version](https://img.shields.io/badge/go-1.21%2B-blue?logo=go)](https://go.dev)
[![Build](https://img.shields.io/github/actions/workflow/status/mikesorae/speqlite/ci.yml?branch=main&label=build)](https://github.com/mikesorae/speqlite/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/mikesorae/speqlite)](https://goreportcard.com/report/github.com/mikesorae/speqlite)

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
go install github.com/mikesorae/speqlite/cmd/speqlite@latest
```

### 5-step workflow

```bash
# Step 1 — Initialise a workspace
speqlite init

# Step 2 — Write a rough spec in Markdown (any structure is fine)
cat > scratch/my-feature.md << 'EOF'
# CMD-IMPORT

Import command parses Markdown files into the spec state.

## CMD-APPLY

Apply command mutates SQLite state from the current plan.

depends_on CMD-IMPORT
EOF

# Step 3 — Import the file and generate a plan (no state mutation yet)
speqlite import scratch/my-feature.md

# Step 4 — Review the pending plan
speqlite plan

# Step 5 — Apply pending changes to SQLite, then render Markdown
speqlite apply
speqlite render --all
```

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   Input Sources                      │
│           Markdown / Plain Text / AI output          │
└─────────────────────────┬───────────────────────────┘
                          │  speqlite import
                          ▼
                    ┌───────────┐
                    │  Importer │  parse headings, IDs, relations
                    └─────┬─────┘
                          │
                          ▼
                    ┌────────────┐
                    │ Normalizer │  assign IDs, infer kinds, hash
                    └─────┬──────┘
                          │
                          ▼
              ┌──────────────────────┐
              │  state.plan.json     │  desired-state diff
              └──────────┬───────────┘
                         │  speqlite apply
                         ▼
              ┌──────────────────────┐
              │  SQLite State        │  canonical source of truth
              │  (.spec/state.sqlite)│
              └──────────┬───────────┘
          ┌──────────────┼──────────────┐
          │              │              │
          ▼              ▼              ▼
      Renderer       Searcher      Validator
   specs/*.md     FTS5 + BM25    cycle detect
```

## Installation

### From source

```bash
git clone https://github.com/mikesorae/speqlite
cd speqlite
make install      # installs to $GOPATH/bin/speqlite
```

### Build locally

```bash
make build        # outputs bin/speqlite
./bin/speqlite --help
```

## Commands

| Command | Description |
|---|---|
| `speqlite init` | Initialise a new workspace (creates `.spec/`, `specs/`, etc.) |
| `speqlite import <file>` | Parse Markdown/plain-text into a plan (no state mutation) |
| `speqlite plan` | Display the pending plan from `.spec/state.plan.json` |
| `speqlite apply` | Apply the pending plan to SQLite state |
| `speqlite render [--all] [--format] [ID]` | Render specs from state to files in `specs/` |
| `speqlite search "<query>"` | Full-text search with FTS5 BM25 ranking |
| `speqlite deps <ID>` | Print the dependency tree for a spec |
| `speqlite validate` | Structural validation (cycles, dangling refs, missing fields) |
| `speqlite state list` | List all specs in state |
| `speqlite state show <ID>` | Show full spec details and event history |
| `speqlite state export` | Export full state snapshot as JSON |

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

After `speqlite init`, the workspace looks like:

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

## Contributing

Contributions are welcome. Please open an issue first to discuss significant changes.

- Fork the repo and create a feature branch (`feature/my-change`)
- Write tests alongside implementation (TDD preferred)
- Run `make test` and `make lint` before submitting a pull request
- Follow [Conventional Commits](https://www.conventionalcommits.org/) for commit messages

## Roadmap

| Milestone | Description |
|---|---|
| **MCP Server** | Expose `spec_get`, `spec_search`, `spec_deps`, `spec_validate` as MCP tools so AI agents can query spec state directly |
| **Formal Export** | `speqlite export --format alloy\|smtlib\|tla` — export constraints and relations to formal verification languages (Alloy, SMT-LIB, TLA+) |
| **Watch Mode** | `speqlite watch` — automatically re-import and re-plan on file change |
| **Diff Viewer** | Richer `plan` output with side-by-side before/after rendering |
