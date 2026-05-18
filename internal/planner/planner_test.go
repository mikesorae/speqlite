package planner_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/normalizer"
	"github.com/mikesorae/speqlite/internal/planner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeNS(id, title, kind, status, hash, body string) normalizer.NormalizedSpec {
	return normalizer.NormalizedSpec{
		ID:     id,
		Title:  title,
		Kind:   kind,
		Status: status,
		Hash:   hash,
		Body:   body,
	}
}

func makeDBSpec(id, title, kind, status, hash string) db.Spec {
	return db.Spec{
		ID:        id,
		Title:     title,
		Kind:      kind,
		Status:    status,
		Version:   1,
		Body:      "body",
		Hash:      hash,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Diff — create
// ---------------------------------------------------------------------------

func TestDiff_Create(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "hash1", "body1"),
	}
	current := []db.Spec{}

	plan := planner.Diff(desired, current)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, planner.ActionCreate, plan.Entries[0].Action)
	assert.Equal(t, "CMD-IMPORT", plan.Entries[0].SpecID)
	assert.Nil(t, plan.Entries[0].Before)
	assert.NotNil(t, plan.Entries[0].After)
	assert.Equal(t, "CMD-IMPORT", plan.Entries[0].After.ID)
}

func TestDiff_MultipleCreates(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "h1", "b1"),
		makeNS("CMD-APPLY", "Apply", "command", "draft", "h2", "b2"),
		makeNS("CMD-RENDER", "Render", "command", "draft", "h3", "b3"),
	}

	plan := planner.Diff(desired, nil)
	creates, updates, deletes := plan.Summary()
	assert.Equal(t, 3, creates)
	assert.Equal(t, 0, updates)
	assert.Equal(t, 0, deletes)
}

// ---------------------------------------------------------------------------
// Diff — no change
// ---------------------------------------------------------------------------

func TestDiff_NoChange(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "samehash", "body"),
	}
	current := []db.Spec{
		makeDBSpec("CMD-IMPORT", "Import", "command", "draft", "samehash"),
	}

	plan := planner.Diff(desired, current)
	assert.True(t, plan.IsEmpty())
}

// ---------------------------------------------------------------------------
// Diff — update
// ---------------------------------------------------------------------------

func TestDiff_Update(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import Updated", "command", "draft", "newhash", "new body"),
	}
	current := []db.Spec{
		makeDBSpec("CMD-IMPORT", "Import", "command", "draft", "oldhash"),
	}

	plan := planner.Diff(desired, current)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, planner.ActionUpdate, plan.Entries[0].Action)
	assert.Equal(t, "CMD-IMPORT", plan.Entries[0].SpecID)
	assert.NotNil(t, plan.Entries[0].Before)
	assert.NotNil(t, plan.Entries[0].After)
	assert.Equal(t, "oldhash", plan.Entries[0].Before.Hash)
	assert.Equal(t, "newhash", plan.Entries[0].After.Hash)
}

// ---------------------------------------------------------------------------
// Diff — delete
// ---------------------------------------------------------------------------

func TestDiff_Delete(t *testing.T) {
	desired := []normalizer.NormalizedSpec{}
	current := []db.Spec{
		makeDBSpec("CMD-OLD", "Old Command", "command", "deprecated", "hash1"),
	}

	plan := planner.Diff(desired, current)
	require.Len(t, plan.Entries, 1)
	assert.Equal(t, planner.ActionDelete, plan.Entries[0].Action)
	assert.Equal(t, "CMD-OLD", plan.Entries[0].SpecID)
	assert.NotNil(t, plan.Entries[0].Before)
	assert.Nil(t, plan.Entries[0].After)
}

// ---------------------------------------------------------------------------
// Diff — mixed
// ---------------------------------------------------------------------------

func TestDiff_Mixed(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import Updated", "command", "draft", "newhash", "new body"),
		makeNS("CMD-NEW", "New Command", "command", "draft", "newhash2", "body2"),
	}
	current := []db.Spec{
		makeDBSpec("CMD-IMPORT", "Import", "command", "draft", "oldhash"),
		makeDBSpec("CMD-OLD", "Old Command", "command", "draft", "oldhash2"),
	}

	plan := planner.Diff(desired, current)
	creates, updates, deletes := plan.Summary()
	assert.Equal(t, 1, creates)
	assert.Equal(t, 1, updates)
	assert.Equal(t, 1, deletes)
}

// ---------------------------------------------------------------------------
// Plan.IsEmpty
// ---------------------------------------------------------------------------

func TestPlan_IsEmpty(t *testing.T) {
	p := planner.Diff(nil, nil)
	assert.True(t, p.IsEmpty())
}

func TestPlan_IsNotEmpty(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "hash1", "body1"),
	}
	p := planner.Diff(desired, nil)
	assert.False(t, p.IsEmpty())
}

// ---------------------------------------------------------------------------
// Plan.Write and Load
// ---------------------------------------------------------------------------

func TestPlan_WriteAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.plan.json")

	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "hash1", "body1"),
		makeNS("CMD-APPLY", "Apply", "command", "draft", "hash2", "body2"),
	}
	original := planner.Diff(desired, nil)

	err := original.Write(path)
	require.NoError(t, err)

	// File must exist.
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load and compare.
	loaded, err := planner.Load(path)
	require.NoError(t, err)
	assert.Len(t, loaded.Entries, 2)
	assert.Equal(t, planner.ActionCreate, loaded.Entries[0].Action)
}

func TestPlan_WriteIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.plan.json")

	p := planner.Diff([]normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "hash1", "body1"),
	}, nil)

	require.NoError(t, p.Write(path))

	// No tmp file should remain.
	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err))
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := planner.Load("/nonexistent/state.plan.json")
	require.Error(t, err)
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.plan.json")
	require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))

	_, err := planner.Load(path)
	require.Error(t, err)
}

// ---------------------------------------------------------------------------
// JSON serialisation shape
// ---------------------------------------------------------------------------

func TestPlan_JSONShape(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-IMPORT", "Import", "command", "draft", "hash1", "body1"),
	}
	p := planner.Diff(desired, nil)

	dir := t.TempDir()
	path := filepath.Join(dir, "plan.json")
	require.NoError(t, p.Write(path))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))

	assert.Contains(t, m, "created_at")
	assert.Contains(t, m, "entries")

	entries := m["entries"].([]any)
	require.Len(t, entries, 1)

	entry := entries[0].(map[string]any)
	assert.Equal(t, "create", entry["action"])
	assert.Equal(t, "CMD-IMPORT", entry["spec_id"])
	assert.Contains(t, entry, "after")
	assert.NotContains(t, entry, "before") // omitempty
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func TestPlan_Summary(t *testing.T) {
	desired := []normalizer.NormalizedSpec{
		makeNS("CMD-A", "A", "command", "draft", "h1", "b1"),
		makeNS("CMD-B", "B", "command", "draft", "newhash", "b2"),
	}
	current := []db.Spec{
		makeDBSpec("CMD-B", "B", "command", "draft", "oldhash"),
		makeDBSpec("CMD-C", "C", "command", "draft", "h3"),
	}

	p := planner.Diff(desired, current)
	creates, updates, deletes := p.Summary()
	assert.Equal(t, 1, creates)
	assert.Equal(t, 1, updates)
	assert.Equal(t, 1, deletes)
}
