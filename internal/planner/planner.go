// Package planner diffs normalised specs against the current SQLite state and
// produces a PlanEntry list suitable for serialisation to state.plan.json.
package planner

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/normalizer"
)

// Action describes the type of change a plan entry represents.
type Action string

const (
	ActionCreate Action = "create"
	ActionUpdate Action = "update"
	ActionDelete Action = "delete"
)

// PlanEntry represents a single pending state change.
type PlanEntry struct {
	// Action is one of create, update, delete.
	Action Action `json:"action"`
	// SpecID is the ID of the affected spec.
	SpecID string `json:"spec_id"`
	// Before holds the current state from SQLite (nil for creates).
	Before *PlanSpec `json:"before,omitempty"`
	// After holds the desired state from the normalised import (nil for deletes).
	After *PlanSpec `json:"after,omitempty"`
}

// PlanSpec is a snapshot of a spec's mutable fields used in plan diffs.
type PlanSpec struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Kind   string `json:"kind"`
	Status string `json:"status"`
	Hash   string `json:"hash"`
	Body   string `json:"body"`
}

// Plan is the full plan document written to state.plan.json.
type Plan struct {
	// CreatedAt is the UTC timestamp when the plan was generated.
	CreatedAt time.Time `json:"created_at"`
	// Entries is the ordered list of pending changes.
	Entries []PlanEntry `json:"entries"`
}

// Diff produces a Plan by comparing normalised specs against the current
// database state. The rules are:
//   - A spec present in `desired` but not in `current` → create.
//   - A spec present in both but with a different hash → update.
//   - A spec present in `current` but not in `desired` → delete.
//
// Note: Speclite import never auto-deletes — deletion entries are only
// produced when the caller explicitly passes current specs that have no
// counterpart in the desired list (used by `speqlite plan` re-run).
func Diff(desired []normalizer.NormalizedSpec, current []db.Spec) *Plan {
	// Index current state by ID.
	currentByID := make(map[string]db.Spec, len(current))
	for _, s := range current {
		currentByID[s.ID] = s
	}

	// Index desired by ID.
	desiredByID := make(map[string]normalizer.NormalizedSpec, len(desired))
	for _, ns := range desired {
		desiredByID[ns.ID] = ns
	}

	var entries []PlanEntry

	// Create or update.
	for _, ns := range desired {
		existing, exists := currentByID[ns.ID]
		if !exists {
			entries = append(entries, PlanEntry{
				Action: ActionCreate,
				SpecID: ns.ID,
				After:  planSpecFromNormalized(ns),
			})
			continue
		}
		// Compare hashes — if different, it's an update.
		if existing.Hash != ns.Hash {
			entries = append(entries, PlanEntry{
				Action: ActionUpdate,
				SpecID: ns.ID,
				Before: planSpecFromDB(existing),
				After:  planSpecFromNormalized(ns),
			})
		}
		// If hashes match, no change needed.
	}

	// Delete: specs in current that are not in desired.
	for _, s := range current {
		if _, found := desiredByID[s.ID]; !found {
			entries = append(entries, PlanEntry{
				Action: ActionDelete,
				SpecID: s.ID,
				Before: planSpecFromDB(s),
			})
		}
	}

	return &Plan{
		CreatedAt: time.Now().UTC(),
		Entries:   entries,
	}
}

// Write serialises the plan to path as indented JSON. The write is atomic:
// it uses a temporary file and rename.
func (p *Plan) Write(path string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("planner: marshal plan: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("planner: write plan tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("planner: rename plan: %w", err)
	}
	return nil
}

// Load reads a plan JSON file from path.
func Load(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("planner: read plan %q: %w", path, err)
	}
	var p Plan
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("planner: unmarshal plan: %w", err)
	}
	return &p, nil
}

// Summary returns counts of create, update, and delete entries.
func (p *Plan) Summary() (creates, updates, deletes int) {
	for _, e := range p.Entries {
		switch e.Action {
		case ActionCreate:
			creates++
		case ActionUpdate:
			updates++
		case ActionDelete:
			deletes++
		}
	}
	return
}

// IsEmpty reports whether the plan has no pending changes.
func (p *Plan) IsEmpty() bool {
	return len(p.Entries) == 0
}

// planSpecFromNormalized converts a NormalizedSpec to a PlanSpec.
func planSpecFromNormalized(ns normalizer.NormalizedSpec) *PlanSpec {
	return &PlanSpec{
		ID:     ns.ID,
		Title:  ns.Title,
		Kind:   ns.Kind,
		Status: ns.Status,
		Hash:   ns.Hash,
		Body:   ns.Body,
	}
}

// planSpecFromDB converts a db.Spec to a PlanSpec.
func planSpecFromDB(s db.Spec) *PlanSpec {
	return &PlanSpec{
		ID:     s.ID,
		Title:  s.Title,
		Kind:   s.Kind,
		Status: s.Status,
		Hash:   s.Hash,
		Body:   s.Body,
	}
}
