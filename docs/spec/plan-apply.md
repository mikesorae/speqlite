# Plan & Apply

Speclite follows a Terraform-inspired plan/apply workflow. No mutation of canonical state occurs without a reviewable plan.

---

## Workflow Overview

```
speclite import scratch.md
  │
  ▼ writes
.spec/state.plan.json
  │
  ▼ reviewed by
speclite plan
  │
  ▼ committed by
speclite apply
  │
  ▼ updates
.spec/state.sqlite
.spec/state.snapshot.json
  │  removes
.spec/state.plan.json
```

---

## `state.plan.json` — JSON Schema

The plan file is a JSON document describing a set of ordered operations to be applied atomically.

### Top-Level Schema

```json
{
  "$schema": "https://speclite.dev/schemas/plan/v1.json",
  "version": 1,
  "plan_hash": "<sha256 of canonical plan serialisation>",
  "generated_at": "<ISO-8601 UTC>",
  "source_files": ["scratch/feature.md"],
  "snapshot_hash": "<hash of state.snapshot.json at plan time>",
  "ops": [ ... ]
}
```

| Field | Type | Description |
|---|---|---|
| `version` | integer | Schema version; currently `1` |
| `plan_hash` | string | SHA-256 of `ops` array serialised as compact JSON |
| `generated_at` | string | ISO-8601 UTC timestamp of plan generation |
| `source_files` | string[] | Files that were imported to produce this plan |
| `snapshot_hash` | string | Hash of `state.snapshot.json` at the time the plan was created. Used for conflict detection at apply time. |
| `ops` | Op[] | Ordered list of operations |

### Operation Object

Each element of `ops` is an operation:

```json
{
  "op": "create" | "update" | "delete" | "transition",
  "id": "<spec-id>",
  ...op-specific fields...
}
```

#### `create` Operation

Creates a new spec node.

```json
{
  "op": "create",
  "id": "CMD-IMPORT",
  "title": "Import command",
  "kind": "command",
  "status": "draft",
  "body": "The import command reads...",
  "hash": "sha256:abc123...",
  "relations": [
    {"relation": "depends_on", "to_id": "STATE-SQLITE"}
  ]
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `op` | string | yes | `"create"` |
| `id` | string | yes | New spec ID |
| `title` | string | yes | Spec title |
| `kind` | string | yes | Spec kind |
| `status` | string | yes | Initial status (usually `"draft"`) |
| `body` | string | yes | Full body text |
| `hash` | string | yes | SHA-256 of canonical serialisation |
| `relations` | Relation[] | no | Relations to create alongside the spec |

#### `update` Operation

Updates an existing spec node's mutable fields.

```json
{
  "op": "update",
  "id": "CMD-RENDER",
  "expected_version": 2,
  "expected_hash": "sha256:old123...",
  "fields": {
    "title": "Render command",
    "body": "Updated body text...",
    "hash": "sha256:new456..."
  }
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `op` | string | yes | `"update"` |
| `id` | string | yes | Spec ID to update |
| `expected_version` | integer | yes | Version at plan time; used for conflict detection |
| `expected_hash` | string | yes | Hash at plan time; used for conflict detection |
| `fields` | object | yes | Fields to update (only changed fields included) |

Updatable fields: `title`, `body`, `hash`. The `kind` and `id` are immutable.

#### `delete` Operation

Marks a spec as deprecated (hard delete is not supported in MVP).

```json
{
  "op": "delete",
  "id": "CMD-OLD",
  "expected_version": 1,
  "mode": "deprecate"
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `op` | string | yes | `"delete"` |
| `id` | string | yes | Spec ID to deprecate |
| `expected_version` | integer | yes | Version at plan time |
| `mode` | string | yes | Currently only `"deprecate"` |

#### `transition` Operation

Transitions a spec to a new lifecycle status.

```json
{
  "op": "transition",
  "id": "CMD-IMPORT",
  "from_status": "draft",
  "to_status": "review",
  "expected_version": 1
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `op` | string | yes | `"transition"` |
| `id` | string | yes | Spec ID |
| `from_status` | string | yes | Expected current status (for conflict detection) |
| `to_status` | string | yes | Target status |
| `expected_version` | integer | yes | Version at plan time |

#### `add_relation` Operation

Adds a relation edge.

```json
{
  "op": "add_relation",
  "from_id": "CMD-APPLY",
  "relation": "depends_on",
  "to_id": "CMD-PLAN"
}
```

#### `remove_relation` Operation

Removes a relation edge.

```json
{
  "op": "remove_relation",
  "from_id": "CMD-APPLY",
  "relation": "depends_on",
  "to_id": "CMD-OLD"
}
```

---

## Complete `state.plan.json` Example

```json
{
  "$schema": "https://speclite.dev/schemas/plan/v1.json",
  "version": 1,
  "plan_hash": "sha256:deadbeef...",
  "generated_at": "2024-01-15T10:00:00Z",
  "source_files": ["scratch/feature.md"],
  "snapshot_hash": "sha256:cafebabe...",
  "ops": [
    {
      "op": "create",
      "id": "CMD-IMPORT",
      "title": "Import command",
      "kind": "command",
      "status": "draft",
      "body": "The import command reads Markdown or plain-text files...",
      "hash": "sha256:abc111...",
      "relations": [
        {"relation": "depends_on", "to_id": "STATE-SQLITE"},
        {"relation": "implements", "to_id": "FR-001"}
      ]
    },
    {
      "op": "update",
      "id": "CMD-RENDER",
      "expected_version": 2,
      "expected_hash": "sha256:old999...",
      "fields": {
        "body": "Updated render command description...",
        "hash": "sha256:new888..."
      }
    },
    {
      "op": "delete",
      "id": "CMD-OLD",
      "expected_version": 1,
      "mode": "deprecate"
    }
  ]
}
```

---

## `state.snapshot.json` — JSON Schema

The snapshot is a point-in-time export of the full state. It is updated by `apply` and used by the planner to compute diffs.

### Top-Level Schema

```json
{
  "$schema": "https://speclite.dev/schemas/snapshot/v1.json",
  "version": 1,
  "snapshot_hash": "<sha256 of content>",
  "taken_at": "<ISO-8601 UTC>",
  "specs": [ ... ],
  "relations": [ ... ],
  "constraints": [ ... ]
}
```

### Spec Entry in Snapshot

```json
{
  "id": "CMD-IMPORT",
  "title": "Import command",
  "kind": "command",
  "status": "fixed",
  "version": 3,
  "hash": "sha256:abc123...",
  "created_at": "2024-01-10T09:00:00Z",
  "updated_at": "2024-01-15T10:30:00Z"
}
```

Note: `body` is **not** included in the snapshot (it is in SQLite). The snapshot carries only the metadata needed for diff computation. This keeps the snapshot file small and Git-diffable.

### Relation Entry in Snapshot

```json
{"from_id": "CMD-APPLY", "relation": "depends_on", "to_id": "CMD-PLAN"}
```

### Constraint Entry in Snapshot

```json
{"id": "CONSTR-001", "target_id": "FR-001", "language": "natural", "expression": "..."}
```

---

## Conflict Detection

When `apply` is called, it compares the current `snapshot_hash` in `state.sqlite` against the `snapshot_hash` recorded in `state.plan.json`.

If they differ, a concurrent mutation has occurred since the plan was generated. The apply engine aborts with exit code `4`.

Additionally, for each `update` and `transition` operation, the apply engine verifies `expected_version` matches the current `version` in `state.sqlite`. If any version has changed, the operation is a conflict.

To resolve conflicts:
1. Inspect the current state: `speclite state show <id>`
2. Re-run `speclite import` to regenerate the plan against current state
3. Review the new plan: `speclite plan`
4. Apply: `speclite apply`

---

## Apply Engine Execution Order

Operations within a plan are applied in the following order to satisfy referential integrity:

1. `create` operations (all new specs created first)
2. `add_relation` operations (relations referencing newly created specs)
3. `update` operations
4. `transition` operations
5. `remove_relation` operations
6. `delete` (deprecate) operations

All operations execute within a single SQLite transaction. If any operation fails, the entire transaction is rolled back and the state is unchanged.

---

## Planner Diff Algorithm

The planner computes the minimal set of operations to transition from snapshot state to the normalizer's output.

```
func Plan(snapshot []SpecNode, desired []SpecNode) []Op:
    ops = []

    snapshotByID = index(snapshot, by=id)
    desiredByID  = index(desired,  by=id)

    for node in desired:
        existing = snapshotByID[node.id]
        if existing == nil:
            ops += CreateOp(node)
        elif existing.hash != node.hash:
            ops += UpdateOp(existing, node)
        // else: unchanged, no op

    for node in snapshot:
        if desiredByID[node.id] == nil:
            if node.status != "deprecated":
                ops += DeleteOp(node)   // mode=deprecate

    // Relation diff
    snapshotRels = set(snapshot.relations)
    desiredRels  = set(desired.relations)
    for rel in desiredRels - snapshotRels:
        ops += AddRelationOp(rel)
    for rel in snapshotRels - desiredRels:
        ops += RemoveRelationOp(rel)

    return ops
```

The planner never generates `transition` operations automatically — those are only produced by explicit `speclite state transition` calls.
