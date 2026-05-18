# Speclite

**AI-native local-first specification state management CLI.**

Speclite treats specifications as *state*, not documents. Inspired by Terraform's plan/apply workflow, it provides a safe, reviewable pipeline for evolving structured specifications from unstructured prose.

---

## Core Philosophy

### Markdown is not canonical

Markdown is an *editing projection* — a human-friendly view of state. The canonical source of truth is the SQLite database.

```
Markdown / Plain Text
  ↓  import
Normalized Spec State
  ↓  plan
Diff / Pending Changes
  ↓  apply
SQLite State
  ↓  render
Markdown / Plain Text
```

Markdown is always regeneratable from state. Never edit rendered Markdown as your primary workflow; import it back through the normalizer instead.

### Specification is stateful

A specification is not a document — it is a node in a state machine with a defined lifecycle:

```
draft → review → fixed → implemented → verified → deprecated
```

Every transition is recorded in the immutable event log.

### AI-first workflow

Speclite does not require strictly structured input. You can write rough plain text or Markdown, and the normalizer will infer IDs, titles, kinds, and relations. This makes it suitable for AI-generated content that needs to be integrated into a managed specification baseline.

### Preserve formalizability

The data model is designed so that specifications can eventually be exported to formal languages (Alloy, TLA+, SMT-LIB, Lean, Datalog) for model checking and theorem proving. This is a non-goal for the MVP but is a first-class architectural concern.

---

## Design Principles (Terraform Analogy)

| Terraform concept | Speclite equivalent |
|---|---|
| Desired infrastructure | Markdown / plain-text spec files |
| State file | `state.sqlite` |
| Plan | `state.plan.json` |
| Apply | State transition via `speqlite apply` |
| Output | `speqlite render` |

---

## Quick Start

```bash
# Initialise a new Speclite workspace
speqlite init

# Import rough Markdown into a pending plan
speqlite import scratch/my-feature.md

# Review the pending changes
speqlite plan

# Commit the plan to state
speqlite apply

# Regenerate Markdown projections from state
speqlite render --all

# Full-text search
speqlite search "plan apply"

# Inspect dependency graph
speqlite deps CMD-APPLY

# Validate structural integrity
speqlite validate
```

---

## Primary Goals

- **Local-first** — everything runs offline, zero cloud dependency
- **CLI-only** — composable with shell scripts and CI pipelines
- **Git-friendly** — state snapshot and plan are plain JSON, diffable
- **AI-readable** — structured JSON and Markdown outputs feed LLM context windows
- **Lightweight** — single binary, embedded SQLite, no daemon
- **Structured specification state** — typed nodes, typed relations, constraints
- **Full-text search** — SQLite FTS5 with BM25 ranking
- **Dependency tracking** — `depends_on`, `implements`, `verifies`, `supersedes`, `related_to`
- **Plan/apply workflow** — no mutation without review
- **Markdown projection** — human-editable, regeneratable

## Non-Goals (MVP)

- Web UI
- SaaS / multi-user collaboration
- RBAC
- Real-time sync
- Graph / vector databases
- Embedding / semantic search
- Automatic proof generation
- Workflow engine
- External integrations (CI webhooks, etc.)

---

## Navigation

| Section | Description |
|---|---|
| [Data Model](spec/data-model.md) | SQLite schema: tables, columns, constraints, FTS |
| [State Machine](spec/state-machine.md) | Spec lifecycle: states, transitions, rules |
| [CLI Reference](spec/cli-reference.md) | Every command, flag, exit code, and example |
| [Normalizer](spec/normalizer.md) | How prose is parsed into structured spec nodes |
| [Plan & Apply](spec/plan-apply.md) | Plan diff format and apply workflow |
| [File Structure](spec/file-structure.md) | `.spec/`, `specs/`, `scratch/` layout |
| [Future Extensions](spec/future-extensions.md) | MCP server and formal language export |
| [Implementation Plan](dev/implementation-plan.md) | Go packages, dependencies, build instructions |
