package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestDB opens a temporary in-memory (or temp file) SQLite database for testing.
func openTestDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")
	d, err := Open(path)
	require.NoError(t, err, "Open should succeed")
	t.Cleanup(func() { d.Close() })
	return d
}

func TestOpenAndMigrate(t *testing.T) {
	d := openTestDB(t)
	assert.NotNil(t, d)

	ver, err := d.userVersion()
	require.NoError(t, err)
	assert.Equal(t, schemaVersion, ver, "user_version should match schemaVersion after migration")
}

func TestOpen_Idempotent(t *testing.T) {
	// Opening an already-migrated database should not error.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.sqlite")

	d1, err := Open(path)
	require.NoError(t, err)
	d1.Close()

	d2, err := Open(path)
	require.NoError(t, err)
	defer d2.Close()

	ver, err := d2.userVersion()
	require.NoError(t, err)
	assert.Equal(t, schemaVersion, ver)
}

func TestInit(t *testing.T) {
	d := openTestDB(t)
	require.NoError(t, d.Init())

	events, err := d.ListEventsByType("init_workspace")
	require.NoError(t, err)
	assert.Len(t, events, 1)
	assert.Contains(t, events[0].PayloadJSON, `"version"`)
}

func makeSpec(id string) Spec {
	now := time.Now().UTC().Truncate(time.Second)
	return Spec{
		ID:        id,
		Title:     "Test Spec " + id,
		Kind:      "requirement",
		Status:    "draft",
		Version:   1,
		Body:      "This is the body of " + id,
		Hash:      CanonicalHash(id, "Test Spec "+id, "requirement", "draft", "This is the body of "+id),
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// --- Specs tests ---

func TestCreateAndGetSpec(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")

	require.NoError(t, d.CreateSpec(s))

	got, err := d.GetSpec("FR-001")
	require.NoError(t, err)
	assert.Equal(t, s.ID, got.ID)
	assert.Equal(t, s.Title, got.Title)
	assert.Equal(t, s.Kind, got.Kind)
	assert.Equal(t, s.Status, got.Status)
	assert.Equal(t, s.Version, got.Version)
	assert.Equal(t, s.Body, got.Body)
	assert.Equal(t, s.Hash, got.Hash)
}

func TestGetSpec_NotFound(t *testing.T) {
	d := openTestDB(t)
	_, err := d.GetSpec("NONEXISTENT-001")
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListSpecs(t *testing.T) {
	d := openTestDB(t)

	specs := []Spec{
		makeSpec("FR-001"),
		makeSpec("FR-002"),
	}
	for _, s := range specs {
		require.NoError(t, d.CreateSpec(s))
	}

	all, err := d.ListSpecs("", "")
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestListSpecs_FilterByKind(t *testing.T) {
	d := openTestDB(t)

	s1 := makeSpec("FR-001")
	s2 := makeSpec("CMD-001")
	s2.Kind = "command"
	s2.Hash = CanonicalHash(s2.ID, s2.Title, s2.Kind, s2.Status, s2.Body)
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	filtered, err := d.ListSpecs("command", "")
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "CMD-001", filtered[0].ID)
}

func TestListSpecs_FilterByStatus(t *testing.T) {
	d := openTestDB(t)

	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	s2.Status = "fixed"
	s2.Hash = CanonicalHash(s2.ID, s2.Title, s2.Kind, s2.Status, s2.Body)
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	fixed, err := d.ListSpecs("", "fixed")
	require.NoError(t, err)
	assert.Len(t, fixed, 1)
	assert.Equal(t, "FR-002", fixed[0].ID)
}

func TestUpdateSpec(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	s.Title = "Updated Title"
	s.Status = "review"
	s.Body = "Updated body"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)

	require.NoError(t, d.UpdateSpec(s))

	got, err := d.GetSpec("FR-001")
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", got.Title)
	assert.Equal(t, "review", got.Status)
	assert.Equal(t, "Updated body", got.Body)
	assert.Equal(t, 2, got.Version, "version should be incremented on update")
}

func TestDeleteSpec(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	require.NoError(t, d.DeleteSpec("FR-001"))

	_, err := d.GetSpec("FR-001")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSpecExists(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	exists, err := d.SpecExists("FR-001")
	require.NoError(t, err)
	assert.True(t, exists)

	missing, err := d.SpecExists("MISSING-001")
	require.NoError(t, err)
	assert.False(t, missing)
}

func TestCreateSpec_InvalidStatus(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	s.Status = "invalid_status"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)

	err := d.CreateSpec(s)
	assert.Error(t, err, "creating spec with invalid status should fail")
}

// --- Relation tests ---

func TestCreateAndListRelations(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	r := Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}
	require.NoError(t, d.CreateRelation(r))

	from, err := d.ListRelationsFrom("FR-001")
	require.NoError(t, err)
	assert.Len(t, from, 1)
	assert.Equal(t, r, from[0])

	to, err := d.ListRelationsTo("FR-002")
	require.NoError(t, err)
	assert.Len(t, to, 1)
	assert.Equal(t, r, to[0])
}

func TestDeleteRelation(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	r := Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}
	require.NoError(t, d.CreateRelation(r))
	require.NoError(t, d.DeleteRelation("FR-001", "depends_on", "FR-002"))

	from, err := d.ListRelationsFrom("FR-001")
	require.NoError(t, err)
	assert.Empty(t, from)
}

func TestRelationExists(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	r := Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}
	require.NoError(t, d.CreateRelation(r))

	exists, err := d.RelationExists("FR-001", "depends_on", "FR-002")
	require.NoError(t, err)
	assert.True(t, exists)

	missing, err := d.RelationExists("FR-001", "implements", "FR-002")
	require.NoError(t, err)
	assert.False(t, missing)
}

func TestDeleteSpec_CascadeDeletesOutgoingRelation(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))
	require.NoError(t, d.CreateRelation(Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}))

	// Deleting FR-001 should cascade delete its outgoing relations.
	require.NoError(t, d.DeleteSpec("FR-001"))

	to, err := d.ListRelationsTo("FR-002")
	require.NoError(t, err)
	assert.Empty(t, to)
}

func TestDeleteSpec_RestrictIncomingRelation(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))
	require.NoError(t, d.CreateRelation(Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}))

	// Deleting FR-002 (to_id) should be blocked by RESTRICT.
	err := d.DeleteSpec("FR-002")
	assert.Error(t, err, "should not be able to delete spec with incoming RESTRICT relation")
}

// --- Constraint tests ---

func TestCreateAndGetConstraint(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	c := Constraint{
		ID:         "C-001",
		TargetID:   "FR-001",
		Language:   "natural",
		Expression: "The system shall respond within 200ms",
		CreatedAt:  time.Now().UTC().Truncate(time.Second),
	}
	require.NoError(t, d.CreateConstraint(c))

	got, err := d.GetConstraint("C-001")
	require.NoError(t, err)
	assert.Equal(t, c.ID, got.ID)
	assert.Equal(t, c.TargetID, got.TargetID)
	assert.Equal(t, c.Language, got.Language)
	assert.Equal(t, c.Expression, got.Expression)
}

func TestGetConstraint_NotFound(t *testing.T) {
	d := openTestDB(t)
	_, err := d.GetConstraint("NONEXISTENT")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListConstraints(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	for i, lang := range []string{"natural", "alloy", "smtlib"} {
		c := Constraint{
			ID:         fmt.Sprintf("C-%03d", i+1),
			TargetID:   "FR-001",
			Language:   lang,
			Expression: "expr",
		}
		require.NoError(t, d.CreateConstraint(c))
	}

	cs, err := d.ListConstraints("FR-001")
	require.NoError(t, err)
	assert.Len(t, cs, 3)
}

func TestDeleteConstraint(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	c := Constraint{ID: "C-001", TargetID: "FR-001", Language: "natural", Expression: "x"}
	require.NoError(t, d.CreateConstraint(c))
	require.NoError(t, d.DeleteConstraint("C-001"))

	_, err := d.GetConstraint("C-001")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestDeleteSpec_CascadeDeletesConstraints(t *testing.T) {
	d := openTestDB(t)
	s := makeSpec("FR-001")
	require.NoError(t, d.CreateSpec(s))

	c := Constraint{ID: "C-001", TargetID: "FR-001", Language: "natural", Expression: "x"}
	require.NoError(t, d.CreateConstraint(c))

	require.NoError(t, d.DeleteSpec("FR-001"))

	_, err := d.GetConstraint("C-001")
	assert.ErrorIs(t, err, ErrNotFound)
}

// --- Event log tests ---

func TestAppendAndListEvents(t *testing.T) {
	d := openTestDB(t)
	specID := "FR-001"

	require.NoError(t, d.AppendEvent("create_spec", &specID, `{"id":"FR-001"}`))
	require.NoError(t, d.AppendEvent("update_spec", &specID, `{"id":"FR-001","fields_changed":["body"]}`))
	require.NoError(t, d.AppendEvent("apply_plan", nil, `{"plan_hash":"abc","ops_count":1}`))

	all, err := d.ListEvents("")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	bySpec, err := d.ListEvents("FR-001")
	require.NoError(t, err)
	assert.Len(t, bySpec, 2)
}

func TestGetEvent(t *testing.T) {
	d := openTestDB(t)
	specID := "FR-001"
	require.NoError(t, d.AppendEvent("create_spec", &specID, `{"id":"FR-001"}`))

	events, err := d.ListEvents("")
	require.NoError(t, err)
	require.Len(t, events, 1)

	got, err := d.GetEvent(events[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "create_spec", got.EventType)
	assert.NotNil(t, got.SpecID)
	assert.Equal(t, "FR-001", *got.SpecID)
}

func TestListEventsByType(t *testing.T) {
	d := openTestDB(t)
	specID := "FR-001"
	require.NoError(t, d.AppendEvent("create_spec", &specID, `{}`))
	require.NoError(t, d.AppendEvent("update_spec", &specID, `{}`))
	require.NoError(t, d.AppendEvent("create_spec", &specID, `{}`))

	events, err := d.ListEventsByType("create_spec")
	require.NoError(t, err)
	assert.Len(t, events, 2)
}

// --- Snapshot tests ---

func TestTakeAndLoadSnapshot(t *testing.T) {
	d := openTestDB(t)

	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))
	require.NoError(t, d.CreateRelation(Relation{FromID: "FR-001", Relation: "depends_on", ToID: "FR-002"}))

	dir := t.TempDir()
	snapPath := filepath.Join(dir, "state.snapshot.json")

	snap, err := d.TakeSnapshot(snapPath)
	require.NoError(t, err)
	assert.Len(t, snap.Specs, 2)
	assert.Len(t, snap.Relations, 1)

	// File should exist and be loadable.
	info, err := os.Stat(snapPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))

	loaded, err := LoadSnapshot(snapPath)
	require.NoError(t, err)
	assert.Len(t, loaded.Specs, 2)
	assert.Equal(t, schemaVersion, loaded.Version)
}

func TestSnapshotHash(t *testing.T) {
	d := openTestDB(t)
	s1 := makeSpec("FR-001")
	s2 := makeSpec("FR-002")
	require.NoError(t, d.CreateSpec(s1))
	require.NoError(t, d.CreateSpec(s2))

	dir := t.TempDir()
	snapPath := filepath.Join(dir, "snap.json")
	snap, err := d.TakeSnapshot(snapPath)
	require.NoError(t, err)

	h1, err := snap.SnapshotHash()
	require.NoError(t, err)
	assert.NotEmpty(t, h1)

	// Same snapshot should produce the same hash.
	h2, err := snap.SnapshotHash()
	require.NoError(t, err)
	assert.Equal(t, h1, h2)
}

func TestCanonicalHash(t *testing.T) {
	h1 := CanonicalHash("FR-001", "Title", "requirement", "draft", "body")
	h2 := CanonicalHash("FR-001", "Title", "requirement", "draft", "body")
	h3 := CanonicalHash("FR-001", "Different Title", "requirement", "draft", "body")

	assert.Equal(t, h1, h2, "same inputs should produce same hash")
	assert.NotEqual(t, h1, h3, "different inputs should produce different hash")
	assert.Len(t, h1, 64, "SHA-256 hex should be 64 chars")
}

// --- FTS tests ---

func TestFTSSync(t *testing.T) {
	d := openTestDB(t)

	s := makeSpec("FR-001")
	s.Title = "unique title bzzzt"
	s.Body = "some distinctive content qwerty"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)
	require.NoError(t, d.CreateSpec(s))

	// Query the FTS index directly.
	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM specs_fts WHERE specs_fts MATCH ?`, "bzzzt").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "FTS should find inserted spec")
}

func TestFTSSync_OnDelete(t *testing.T) {
	d := openTestDB(t)

	s := makeSpec("FR-001")
	s.Title = "unique title xyzzy"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)
	require.NoError(t, d.CreateSpec(s))
	require.NoError(t, d.DeleteSpec("FR-001"))

	var count int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM specs_fts WHERE specs_fts MATCH ?`, "xyzzy").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "FTS should remove deleted spec")
}

func TestFTSSync_OnUpdate(t *testing.T) {
	d := openTestDB(t)

	s := makeSpec("FR-001")
	s.Title = "original title aaabbb"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)
	require.NoError(t, d.CreateSpec(s))

	s.Title = "updated title cccddd"
	s.Hash = CanonicalHash(s.ID, s.Title, s.Kind, s.Status, s.Body)
	require.NoError(t, d.UpdateSpec(s))

	var oldCount, newCount int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM specs_fts WHERE specs_fts MATCH ?`, "aaabbb").Scan(&oldCount)
	require.NoError(t, err)
	assert.Equal(t, 0, oldCount, "FTS should not find old title after update")

	err = d.db.QueryRow(`SELECT COUNT(*) FROM specs_fts WHERE specs_fts MATCH ?`, "cccddd").Scan(&newCount)
	require.NoError(t, err)
	assert.Equal(t, 1, newCount, "FTS should find new title after update")
}

