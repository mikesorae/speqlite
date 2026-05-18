package apply_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikesorae/speqlite/internal/apply"
	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openTestDB(t *testing.T) *db.DB {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "test.sqlite")
	database, err := db.Open(tmp)
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })
	return database
}

func TestApply_Create(t *testing.T) {
	database := openTestDB(t)
	snapPath := filepath.Join(t.TempDir(), "snap.json")

	plan := &planner.Plan{
		CreatedAt: time.Now().UTC(),
		Entries: []planner.PlanEntry{
			{
				Action: planner.ActionCreate,
				SpecID: "CMD-APPLY",
				After: &planner.PlanSpec{
					ID:     "CMD-APPLY",
					Title:  "Apply Command",
					Kind:   "command",
					Status: "draft",
					Hash:   "abc123",
					Body:   "Applies pending plan.",
				},
			},
		},
	}

	result, err := apply.Apply(database, plan, snapPath)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 0, result.Deleted)
	assert.Equal(t, 1, result.Applied)

	spec, err := database.GetSpec("CMD-APPLY")
	require.NoError(t, err)
	assert.Equal(t, "Apply Command", spec.Title)
	assert.Equal(t, "command", spec.Kind)

	// Snapshot should exist.
	_, err = os.Stat(snapPath)
	assert.NoError(t, err)
}

func TestApply_Update(t *testing.T) {
	database := openTestDB(t)
	snapPath := filepath.Join(t.TempDir(), "snap.json")

	// Pre-create a spec.
	require.NoError(t, database.CreateSpec(db.Spec{
		ID:     "REQ-1",
		Title:  "Old Title",
		Kind:   "requirement",
		Status: "draft",
		Body:   "Old body",
		Hash:   "oldhash",
	}))

	plan := &planner.Plan{
		CreatedAt: time.Now().UTC(),
		Entries: []planner.PlanEntry{
			{
				Action: planner.ActionUpdate,
				SpecID: "REQ-1",
				Before: &planner.PlanSpec{
					ID:   "REQ-1",
					Hash: "oldhash",
				},
				After: &planner.PlanSpec{
					ID:     "REQ-1",
					Title:  "New Title",
					Kind:   "requirement",
					Status: "review",
					Hash:   "newhash",
					Body:   "New body",
				},
			},
		},
	}

	result, err := apply.Apply(database, plan, snapPath)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 0, result.Deleted)

	spec, err := database.GetSpec("REQ-1")
	require.NoError(t, err)
	assert.Equal(t, "New Title", spec.Title)
	assert.Equal(t, "review", spec.Status)
}

func TestApply_Delete(t *testing.T) {
	database := openTestDB(t)
	snapPath := filepath.Join(t.TempDir(), "snap.json")

	require.NoError(t, database.CreateSpec(db.Spec{
		ID:     "OLD-SPEC",
		Title:  "Old Spec",
		Kind:   "requirement",
		Status: "deprecated",
		Body:   "Old",
		Hash:   "delhash",
	}))

	plan := &planner.Plan{
		CreatedAt: time.Now().UTC(),
		Entries: []planner.PlanEntry{
			{
				Action: planner.ActionDelete,
				SpecID: "OLD-SPEC",
				Before: &planner.PlanSpec{
					ID:   "OLD-SPEC",
					Hash: "delhash",
				},
			},
		},
	}

	result, err := apply.Apply(database, plan, snapPath)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 1, result.Deleted)

	_, err = database.GetSpec("OLD-SPEC")
	assert.Error(t, err)
}

func TestApply_EventLog(t *testing.T) {
	database := openTestDB(t)
	snapPath := filepath.Join(t.TempDir(), "snap.json")

	plan := &planner.Plan{
		CreatedAt: time.Now().UTC(),
		Entries: []planner.PlanEntry{
			{
				Action: planner.ActionCreate,
				SpecID: "CMD-TEST",
				After: &planner.PlanSpec{
					ID:     "CMD-TEST",
					Title:  "Test Command",
					Kind:   "command",
					Status: "draft",
					Hash:   "testhash",
					Body:   "Test body",
				},
			},
		},
	}

	_, err := apply.Apply(database, plan, snapPath)
	require.NoError(t, err)

	events, err := database.ListEvents("CMD-TEST")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "create_spec", events[0].EventType)
}

func TestApply_EmptyPlan(t *testing.T) {
	database := openTestDB(t)
	snapPath := filepath.Join(t.TempDir(), "snap.json")

	plan := &planner.Plan{
		CreatedAt: time.Now().UTC(),
		Entries:   nil,
	}

	result, err := apply.Apply(database, plan, snapPath)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Applied)
}
