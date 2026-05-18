# Normalizer Algorithm

The normalizer converts unstructured Markdown or plain text into a list of `SpecNode` values that can be diffed against the current state. It is the core of the AI-friendly import pipeline.

---

## Overview

```
Input file (Markdown / plain text)
  │
  ▼
┌─────────────┐
│   Importer  │   Detect file type, split into raw sections
└──────┬──────┘
       │  []RawSection
       ▼
┌──────────────┐
│   Normalizer │   Extract IDs, titles, kinds, relations, body
└──────┬───────┘
       │  []SpecNode
       ▼
┌─────────────┐
│   Planner   │   Diff against state snapshot → Plan ops
└─────────────┘
```

The normalizer is **idempotent**: running it twice on the same input produces the same output.

---

## Phase 1: Section Splitting

The importer splits the input into *raw sections*. A section is a contiguous block of text that is expected to represent a single spec node.

### Markdown Splitting Rules

The Goldmark parser is used to parse the Markdown AST. Section boundaries are determined as follows:

1. **Heading-based splitting**: Each heading at the configured split level (default: H2) starts a new section. The heading text becomes the section title candidate.
2. **Horizontal rule splitting**: A `---` or `***` at the top level can optionally split sections (disabled by default; enabled with `--split-on-hr`).
3. **Front matter**: YAML front matter (between `---` delimiters at the top of the file) is parsed separately and applies to the first section or to all sections if it contains a `global: true` key.

#### Example Markdown

```markdown
# My Feature Spec

This is the overview.

## CMD-IMPORT: Import Command

Reads Markdown files and generates a plan.

**Depends on**: STATE-SQLITE, FR-001

## FR-001: Import Functional Requirement

The system must support importing Markdown files.
```

This is split into 3 sections:
- Section 0: `# My Feature Spec` (file-level, becomes a `note` kind if no ID detected)
- Section 1: `## CMD-IMPORT: Import Command`
- Section 2: `## FR-001: Import Functional Requirement`

### Plain Text Splitting Rules

For non-Markdown plain text:

1. **Blank-line block splitting**: Consecutive non-blank lines form a block. Two or more blank lines end a block.
2. **ID-line detection**: If the first line of a block matches the ID pattern, it is treated as a section header.
3. If no sections can be determined, the entire file is treated as one section.

---

## Phase 2: ID Extraction

For each raw section, the normalizer attempts to extract or generate a canonical ID.

### Heuristic Priority Order

The normalizer tries each heuristic in order and uses the first match:

#### Heuristic H1: Explicit ID in Heading

Pattern: heading text matches `^({ID_PATTERN})[:\s-]+(.+)$`

```
ID_PATTERN = [A-Z][A-Z0-9]*(-[A-Z0-9]+)+
```

Examples:
- `## CMD-IMPORT: Import Command` → ID = `CMD-IMPORT`, title = `Import Command`
- `## FR-001 - Functional Requirement` → ID = `FR-001`, title = `Functional Requirement`
- `## STATE-SQLITE` → ID = `STATE-SQLITE`, title = `STATE-SQLITE` (title = ID if no suffix)

#### Heuristic H2: ID in First Body Line

If the heading does not contain an ID, check the first non-empty line of the body:

```
CMD-IMPORT: Import Command
```

Pattern: `^({ID_PATTERN})[:\s]+(.+)$` or `^({ID_PATTERN})$`

#### Heuristic H3: Roundtrip Marker

If the document was previously rendered by `speqlite render`, it contains a roundtrip marker:

```html
<!-- speqlite:id=CMD-IMPORT speqlite:version=3 speqlite:hash=abc123 -->
```

This is parsed as the authoritative ID and version.

#### Heuristic H4: ID Generation

If no ID is found, the normalizer generates one:

```
generated_id = {PREFIX}-{SLUG}
```

Where:
- `PREFIX` is derived from the inferred kind (see Phase 4): `CMD`, `FR`, `ARCH`, `STATE`, `NOTE`, etc.
- `SLUG` is generated from the title using:
  1. Convert to uppercase
  2. Replace non-alphanumeric with `-`
  3. Trim leading/trailing `-`
  4. Truncate to 20 characters
  5. Append a 3-digit sequence number if collision detected: `CMD-IMPORT-001`

#### Heuristic H5: Prefix Override

If `--id-prefix` is passed on the CLI, all generated IDs use that prefix regardless of inferred kind.

---

## Phase 3: Title Inference

If the ID was extracted from the heading (H1), the title is the remainder of the heading text after the ID and separator.

If the ID came from the body or was generated (H2–H5), the title is:

1. The first heading in the section, if present, with the ID removed.
2. The first sentence of the body (up to the first `.`, `!`, or `?`, maximum 80 characters).
3. If nothing is found: the ID itself.

Title normalisation:
- Strip leading/trailing whitespace.
- Collapse internal whitespace to a single space.
- Strip surrounding backticks, asterisks, and quotes.

---

## Phase 4: Kind Classification

The normalizer assigns a `kind` to each section using a rule-based classifier.

### Classification Rules (in priority order)

| Rule | Pattern | Assigned Kind |
|---|---|---|
| Front matter `kind:` field | `kind: command` | Verbatim value |
| Explicit roundtrip marker `speqlite:kind` | `speqlite:kind=command` | Verbatim value |
| `--kind` CLI flag | — | Verbatim value |
| ID prefix `CMD-` | `CMD-*` | `command` |
| ID prefix `FR-` | `FR-*` | `requirement` |
| ID prefix `ARCH-` | `ARCH-*` | `architecture` |
| ID prefix `STATE-` | `STATE-*` | `state` |
| ID prefix `TEST-` | `TEST-*` | `test` |
| ID prefix `CONSTR-` | `CONSTR-*` | `constraint` |
| ID prefix `GLOSS-` | `GLOSS-*` | `glossary` |
| Heading keywords | heading contains `Requirement`, `Must`, `Shall` | `requirement` |
| Heading keywords | heading contains `Command`, `CLI`, `Flag` | `command` |
| Heading keywords | heading contains `Architecture`, `Design`, `Package` | `architecture` |
| Heading keywords | heading contains `State`, `Status`, `Lifecycle` | `state` |
| Default | — | `note` |

---

## Phase 5: Relation Parsing

The normalizer scans the body of each section for relation declarations.

### Inline Relation Syntax

Recognised patterns (case-insensitive):

```markdown
**Depends on**: CMD-PLAN, STATE-SQLITE
**Implements**: FR-001
**Verifies**: FR-003
**Supersedes**: CMD-IMPORT-OLD
**Related to**: ARCH-NORMALIZER
```

Also recognised:
```markdown
- depends_on: CMD-PLAN
- implements: FR-001
```

And YAML front matter:
```yaml
relations:
  depends_on: [CMD-PLAN, STATE-SQLITE]
  implements: [FR-001]
```

### Parsed Relation Object

```go
type Relation struct {
    FromID   string
    Relation string  // one of: depends_on, implements, verifies, supersedes, related_to
    ToID     string
}
```

Relations referencing unknown IDs are preserved in the plan as *pending relations*. The validator will flag them as `E_BROKEN_RELATION` unless the target is also being created in the same plan.

---

## Phase 6: Body Cleaning

After extracting metadata (ID, title, kind, relations), the normalizer produces a cleaned body:

1. Remove the first heading line (used as title).
2. Remove any roundtrip marker HTML comment.
3. Remove relation declaration lines (the metadata is stored in `relations` table separately).
4. Remove YAML front matter block.
5. Strip leading/trailing blank lines.
6. Normalise line endings to `\n`.

The resulting body is stored verbatim in `specs.body`.

---

## Phase 7: Hash Computation

A SHA-256 hash is computed over the canonical serialisation:

```
{id}\n{title}\n{kind}\n{status}\n{body}
```

Note: `status` is included so that a status transition changes the hash and triggers an update event.

---

## Full Normalizer Pseudocode

```
func Normalize(file File, opts Options) ([]SpecNode, error):
    raw = parse(file)                         // Phase 1: split
    sections = splitSections(raw, opts)

    nodes = []
    for section in sections:
        id, title = extractID(section, opts)  // Phase 2 & 3
        if id == "":
            id = generateID(title, opts)

        kind = classifyKind(id, section, opts) // Phase 4

        relations = parseRelations(section)    // Phase 5

        body = cleanBody(section, id, title)   // Phase 6

        hash = sha256(id + title + kind + status + body) // Phase 7

        nodes = append(nodes, SpecNode{
            ID:        id,
            Title:     title,
            Kind:      kind,
            Status:    opts.DefaultStatus,  // "draft"
            Body:      body,
            Hash:      hash,
            Relations: relations,
        })

    return deduplicate(nodes), nil
```

---

## Deduplication

If the same ID appears in multiple sections (e.g., the same ID in two files), the normalizer:

1. Merges the bodies (appends later body to first body with a separator).
2. Merges relation sets (union).
3. Logs a warning: `W_DUPLICATE_ID_MERGED`.

---

## Error and Warning Codes

| Code | Level | Description |
|---|---|---|
| `W_DUPLICATE_ID_MERGED` | Warning | Same ID found in multiple sections; bodies merged |
| `W_ID_GENERATED` | Info | ID was generated (not explicitly declared) |
| `W_KIND_DEFAULTED` | Info | Kind defaulted to `note` |
| `W_EMPTY_BODY` | Warning | Section has no body after cleaning |
| `E_INVALID_ID_FORMAT` | Error | Generated ID does not match `ID_PATTERN` |
| `E_RELATION_PARSE` | Warning | Relation line found but could not be fully parsed |
