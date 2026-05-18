// Package apply reads state.plan.json, applies each PlanEntry to SQLite,
// writes event_log entries, and updates the snapshot.
package apply

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/planner"
)

// Result summarises the outcome of an apply operation.
type Result struct {
	Applied  int
	Created  int
	Updated  int
	Deleted  int
	Errors   []string
}

// Apply reads the plan from planPath and applies all entries to the database.
// After all entries are applied it takes a new snapshot at snapshotPath and
// clears the plan file by removing it. Returns a Result with counts.
func Apply(database *db.DB, plan *planner.Plan, snapshotPath string) (*Result, error) {
	result := &Result{}

	for _, entry := range plan.Entries {
		if err := applyEntry(database, entry, result); err != nil {
			return result, fmt.Errorf("apply: entry %s %s: %w", entry.Action, entry.SpecID, err)
		}
	}

	// Take a new snapshot after all changes have been applied.
	if _, err := database.TakeSnapshot(snapshotPath); err != nil {
		return result, fmt.Errorf("apply: take snapshot: %w", err)
	}

	return result, nil
}

// applyEntry applies a single PlanEntry to the database and appends an event.
func applyEntry(database *db.DB, entry planner.PlanEntry, result *Result) error {
	specID := entry.SpecID

	switch entry.Action {
	case planner.ActionCreate:
		if entry.After == nil {
			return fmt.Errorf("create entry %q has nil After", specID)
		}
		spec := db.Spec{
			ID:     entry.After.ID,
			Title:  entry.After.Title,
			Kind:   entry.After.Kind,
			Status: entry.After.Status,
			Body:   entry.After.Body,
			Hash:   entry.After.Hash,
		}
		if err := database.CreateSpec(spec); err != nil {
			return fmt.Errorf("create spec: %w", err)
		}
		payload := eventPayload(map[string]any{
			"id":    specID,
			"title": entry.After.Title,
			"kind":  entry.After.Kind,
		})
		if err := database.AppendEvent("create_spec", &specID, payload); err != nil {
			return fmt.Errorf("append create_spec event: %w", err)
		}
		result.Created++
		result.Applied++

	case planner.ActionUpdate:
		if entry.After == nil {
			return fmt.Errorf("update entry %q has nil After", specID)
		}
		// Fetch current spec to preserve fields not in PlanSpec (e.g. CreatedAt).
		current, err := database.GetSpec(specID)
		if err != nil {
			return fmt.Errorf("get current spec for update: %w", err)
		}
		current.Title = entry.After.Title
		current.Kind = entry.After.Kind
		current.Status = entry.After.Status
		current.Body = entry.After.Body
		current.Hash = entry.After.Hash
		if err := database.UpdateSpec(current); err != nil {
			return fmt.Errorf("update spec: %w", err)
		}
		payload := eventPayload(map[string]any{
			"id":          specID,
			"before_hash": entry.Before.Hash,
			"after_hash":  entry.After.Hash,
		})
		if err := database.AppendEvent("update_spec", &specID, payload); err != nil {
			return fmt.Errorf("append update_spec event: %w", err)
		}
		result.Updated++
		result.Applied++

	case planner.ActionDelete:
		if err := database.DeleteSpec(specID); err != nil {
			return fmt.Errorf("delete spec: %w", err)
		}
		payload := eventPayload(map[string]any{
			"id":        specID,
			"deleted_at": time.Now().UTC().Format(time.RFC3339),
		})
		if err := database.AppendEvent("delete_spec", &specID, payload); err != nil {
			return fmt.Errorf("append delete_spec event: %w", err)
		}
		result.Deleted++
		result.Applied++

	default:
		return fmt.Errorf("unknown action %q", entry.Action)
	}

	return nil
}

// eventPayload marshals a map to a compact JSON string for the event log.
func eventPayload(v map[string]any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
