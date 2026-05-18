# Implementation Plan

This document describes the Go package layout, dependency list, and build instructions for the Speclite MVP.

---

## Module

```
github.com/speclite/speclite
```

Go version: **1.22** or later (for `range over int`, `slices`, `maps` stdlib packages).

---

## Repository Layout

```
github.com/speclite/speclite/
│
├── cmd/
│   └── speclite/
│       └── main.go              ← Binary entry point
│
├── internal/
│   ├── db/                      ← SQLite access layer
│   │   ├── db.go                ← Open/close, migrations, PRAGMA setup
│   │   ├── schema.go            ← Embedded SQL DDL (go:embed)
│   │   ├── specs.go             ← CRUD for specs table
│   │   ├── relations.go         ← CRUD for relations table
│   │   ├── constraints.go       ← CRUD for constraints table
│   │   ├── events.go            ← Append-only writes to event_log
│   │   ├── search.go            ← FTS5 query execution
│   │   └── snapshot.go          ← Snapshot read/write
│   │
│   ├── importer/                ← File reading and dispatch
│   │   ├── importer.go          ← Import(path string) ([]RawSection, error)
│   │   ├── markdown.go          ← Goldmark-based Markdown → RawSection
│   │   └── plaintext.go         ← Plain-text block splitter
│   │
│   ├── normalizer/              ← RawSection → SpecNode
│   │   ├── normalizer.go        ← Normalize([]RawSection, Options) ([]SpecNode, error)
│   │   ├── id.go                ← ID extraction heuristics (H1–H5)
│   │   ├── title.go             ← Title inference
│   │   ├── kind.go              ← Kind classification rules
│   │   ├── relations.go         ← Relation parsing from body
│   │   ├── body.go              ← Body cleaning
│   │   └── hash.go              ← Canonical hash computation
│   │
│   ├── planner/                 ← Diff computation
│   │   ├── planner.go           ← Plan(snapshot, desired) ([]Op, error)
│   │   ├── op.go                ← Op type definitions
│   │   └── plan.go              ← PlanFile read/write (JSON)
│   │
│   ├── apply/                   ← State mutation engine
│   │   ├── apply.go             ← Apply(db, plan) error
│   │   ├── validate.go          ← Pre-apply validation
│   │   └── conflict.go          ← Conflict detection (version/hash check)
│   │
│   ├── renderer/                ← State → Markdown/text/JSON
│   │   ├── renderer.go          ← Render(spec, relations) string
│   │   ├── markdown.go          ← Markdown template
│   │   ├── text.go              ← Plain-text template
│   │   └── json.go              ← JSON serialisation
│   │
│   ├── search/                  ← Search result types and ranking
│   │   ├── search.go            ← Search(db, query, opts) ([]Result, error)
│   │   └── result.go            ← Result type
│   │
│   ├── validator/               ← Structural validation
│   │   ├── validator.go         ← Validate(db) ([]Issue, error)
│   │   ├── rules.go             ← Individual check functions
│   │   └── issue.go             ← Issue type (code, severity, message)
│   │
│   └── workspace/               ← Workspace discovery and config
│       ├── workspace.go         ← Find .spec/, load config
│       └── config.go            ← Config struct and defaults
│
├── schema/
│   └── v1.sql                   ← Full DDL (embedded via go:embed)
│
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Package Responsibilities

### `cmd/speclite`

Entry point. Uses Cobra to define all subcommands. Each subcommand is a thin wrapper that:
1. Resolves the workspace.
2. Opens the database.
3. Calls the appropriate `internal/` package.
4. Formats and prints output.

```go
package main

import (
    "github.com/speclite/speclite/internal/workspace"
    "github.com/spf13/cobra"
)

func main() {
    root := &cobra.Command{Use: "speclite"}
    root.AddCommand(
        newInitCmd(),
        newImportCmd(),
        newPlanCmd(),
        newApplyCmd(),
        newRenderCmd(),
        newSearchCmd(),
        newDepsCmd(),
        newValidateCmd(),
        newStateCmd(),
        newExportCmd(),
        newVersionCmd(),
    )
    root.Execute()
}
```

### `internal/db`

All SQLite interactions. The `DB` struct wraps `*sql.DB` and exposes typed methods.

Key types:
```go
type DB struct { db *sql.DB }

type Spec struct {
    ID        string
    Title     string
    Kind      string
    Status    string
    Version   int
    Body      string
    Hash      string
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Relation struct {
    FromID   string
    Relation string
    ToID     string
}

type Constraint struct {
    ID         string
    TargetID   string
    Language   string
    Expression string
    CreatedAt  time.Time
}

type EventLog struct {
    ID          int64
    EventType   string
    SpecID      *string
    PayloadJSON string
    CreatedAt   time.Time
}
```

The `Open(path string) (*DB, error)` function:
1. Opens the SQLite file.
2. Sets `PRAGMA journal_mode = WAL`.
3. Sets `PRAGMA foreign_keys = ON`.
4. Checks `PRAGMA user_version`.
5. Runs pending migrations if needed.

### `internal/importer`

```go
type RawSection struct {
    Heading    string            // raw heading text (may be empty)
    Body       string            // raw body text
    FrontMatter map[string]any   // parsed YAML front matter, if any
    SourceFile string            // originating file path
    LineStart  int               // line number in source file
}

func Import(path string) ([]RawSection, error)
```

Uses [`yuin/goldmark`](https://github.com/yuin/goldmark) to parse Markdown AST and walk heading nodes for section splitting. For plain text, a simple line-scanner is used.

### `internal/normalizer`

```go
type SpecNode struct {
    ID        string
    Title     string
    Kind      string
    Status    string
    Body      string
    Hash      string
    Relations []RelationSpec
}

type RelationSpec struct {
    Relation string
    ToID     string
}

type Options struct {
    DefaultStatus string
    IDPrefix      string
    ForceKind     string
    SplitOnHR     bool
}

func Normalize(sections []importer.RawSection, opts Options) ([]SpecNode, []Warning, error)
```

### `internal/planner`

```go
type Op struct {
    Op              string          // "create", "update", "delete", "transition", "add_relation", "remove_relation"
    ID              string
    // ...op-specific fields as struct tags
}

type PlanFile struct {
    Version      int
    PlanHash     string
    GeneratedAt  time.Time
    SourceFiles  []string
    SnapshotHash string
    Ops          []Op
}

func Plan(snapshot []db.Spec, snapshotRels []db.Relation, desired []normalizer.SpecNode) (*PlanFile, error)
func WritePlan(path string, plan *PlanFile) error
func ReadPlan(path string) (*PlanFile, error)
```

### `internal/apply`

```go
func Apply(database *db.DB, plan *planner.PlanFile, opts ApplyOptions) error

type ApplyOptions struct {
    DryRun      bool
    AutoApprove bool
}
```

Executes all ops in a single `BEGIN IMMEDIATE` transaction. On success, updates `state.snapshot.json`.

### `internal/renderer`

```go
type Format string
const (
    FormatMarkdown Format = "markdown"
    FormatText     Format = "text"
    FormatJSON     Format = "json"
)

func Render(spec db.Spec, relations []db.Relation, format Format) (string, error)
func RenderAll(database *db.DB, dir string, format Format, filter Filter) error
```

### `internal/search`

```go
type Result struct {
    ID     string
    Title  string
    Kind   string
    Status string
    Rank   float64
}

type Options struct {
    Limit  int
    Kind   string
    Status string
}

func Search(database *db.DB, query string, opts Options) ([]Result, error)
```

### `internal/validator`

```go
type Severity string
const (
    SeverityError   Severity = "error"
    SeverityWarning Severity = "warning"
    SeverityInfo    Severity = "info"
)

type Issue struct {
    Code     string
    Severity Severity
    Message  string
    SpecID   string
}

func Validate(database *db.DB, opts ValidateOptions) ([]Issue, error)
```

### `internal/workspace`

```go
type Workspace struct {
    Root         string
    DBPath       string
    PlanPath     string
    SnapshotPath string
    SpecsDir     string
    ScratchDir   string
}

func Find(start string) (*Workspace, error)   // walks up directory tree to find .spec/
func Init(root string, force bool) error
```

---

## Dependencies

### Direct Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/spf13/cobra` | v1.8.x | CLI command/flag framework |
| `modernc.org/sqlite` | v1.29.x | Pure-Go SQLite driver (no CGO required) |
| `github.com/yuin/goldmark` | v1.7.x | Markdown AST parser |
| `gopkg.in/yaml.v3` | v3.0.x | YAML front matter parsing |
| `github.com/google/uuid` | v1.6.x | UUID generation for constraint IDs |

### Alternative SQLite Driver

If CGO is acceptable (e.g., in a native build environment), `github.com/mattn/go-sqlite3` can be substituted for `modernc.org/sqlite`. The `internal/db` package abstracts this via the standard `database/sql` interface.

To enable CGO build:
```bash
CGO_ENABLED=1 go build ./cmd/speclite
```

To use pure-Go build (default):
```bash
CGO_ENABLED=0 go build ./cmd/speclite
```

The Makefile exposes both targets.

### Test Dependencies

| Package | Version | Purpose |
|---|---|---|
| `github.com/stretchr/testify` | v1.9.x | Assertions and test helpers |

---

## `go.mod`

```
module github.com/speclite/speclite

go 1.22

require (
    github.com/spf13/cobra v1.8.1
    modernc.org/sqlite v1.29.10
    github.com/yuin/goldmark v1.7.4
    gopkg.in/yaml.v3 v3.0.1
    github.com/google/uuid v1.6.0
)

require (
    github.com/stretchr/testify v1.9.0 // test only
)
```

---

## Build Instructions

### Prerequisites

- Go 1.22+
- `make` (optional but recommended)
- No CGO required for default build

### Build

```bash
# Clone
git clone https://github.com/speclite/speclite.git
cd speclite

# Install dependencies
go mod download

# Build binary (pure-Go SQLite)
go build -o bin/speclite ./cmd/speclite

# Or with CGO SQLite (faster for large workspaces)
CGO_ENABLED=1 go build -o bin/speclite-cgo ./cmd/speclite

# Install to $GOPATH/bin
go install ./cmd/speclite
```

### Makefile Targets

```makefile
.PHONY: build test lint clean install

build:
	go build -o bin/speclite ./cmd/speclite

build-cgo:
	CGO_ENABLED=1 go build -o bin/speclite ./cmd/speclite

test:
	go test ./...

test-verbose:
	go test -v ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/ coverage.out coverage.html

install:
	go install ./cmd/speclite
```

### Cross-compilation

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build -o bin/speclite-linux-amd64 ./cmd/speclite

# macOS ARM64
GOOS=darwin GOARCH=arm64 go build -o bin/speclite-darwin-arm64 ./cmd/speclite

# Windows AMD64
GOOS=windows GOARCH=amd64 go build -o bin/speclite-windows-amd64.exe ./cmd/speclite
```

---

## Testing Strategy

### Unit Tests

Each `internal/` package has a corresponding `_test.go` file. Tests use `testify/assert` and `testify/require`.

Key test cases:
- `internal/normalizer`: Table-driven tests for each heuristic with representative inputs (empty, ID-first-heading, plain text, roundtrip marker, generated ID)
- `internal/planner`: Diff correctness (create/update/delete/no-op), relation diffing
- `internal/apply`: Transaction rollback on error, conflict detection, version increment
- `internal/validator`: Each validation rule independently, cyclic dependency detection
- `internal/search`: FTS5 ranking and filtering

### Integration Tests

`internal/db/integration_test.go` uses a temporary SQLite file for end-to-end pipeline tests:

```
import → normalize → plan → apply → render → search
```

### Test Command

```bash
go test ./...
go test -race ./...          # race condition detection
go test -count=1 ./...       # disable test caching
```

---

## Implementation Order (Recommended)

1. `internal/workspace` — workspace discovery
2. `internal/db` — schema, open/close, migrations, basic CRUD
3. `cmd/speclite init` — uses `workspace` and `db`
4. `internal/importer` — Markdown and plain-text parsing
5. `internal/normalizer` — ID/title/kind/relation extraction
6. `internal/planner` — diff algorithm and plan file I/O
7. `cmd/speclite import` — calls importer + normalizer + planner
8. `cmd/speclite plan` — reads plan file, prints summary
9. `internal/apply` — transaction engine
10. `cmd/speclite apply` — calls apply engine
11. `internal/renderer` — Markdown template
12. `cmd/speclite render` — calls renderer
13. `internal/search` — FTS5 wrapper
14. `cmd/speclite search` — calls search
15. `internal/validator` — all validation rules
16. `cmd/speclite validate` — calls validator
17. `cmd/speclite state` — list/show/export/transition subcommands
18. `cmd/speclite deps` — graph traversal
19. `cmd/speclite export` — formal export stubs

---

## Error Handling Convention

All `internal/` functions return `(T, error)`. Errors are wrapped with `fmt.Errorf("package: operation: %w", err)` to preserve the chain.

CLI commands convert errors to user-friendly messages and exit with the appropriate exit code (see [CLI Reference](../spec/cli-reference.md#exit-codes)).

No `panic` in library code. Panics are only acceptable in `main()` for unrecoverable startup failures.

---

## Logging Convention

Use the standard `log/slog` package (Go 1.21+):

```go
slog.Info("applying plan", "ops", len(plan.Ops), "plan_hash", plan.PlanHash)
slog.Error("failed to open database", "path", dbPath, "err", err)
```

Log level is controlled by the `--verbose` flag:
- Default: `slog.LevelWarn`
- `--verbose`: `slog.LevelInfo`
- `--verbose --verbose`: `slog.LevelDebug`
