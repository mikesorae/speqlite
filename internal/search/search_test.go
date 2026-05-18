package search_test

import (
	"path/filepath"
	"testing"

	"github.com/speclite/speclite/internal/db"
	"github.com/speclite/speclite/internal/search"
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

func seedSpecs(t *testing.T, database *db.DB, specs []db.Spec) {
	t.Helper()
	for _, s := range specs {
		require.NoError(t, database.CreateSpec(s))
	}
}

func TestSearch_Basic(t *testing.T) {
	database := openTestDB(t)
	seedSpecs(t, database, []db.Spec{
		{ID: "CMD-APPLY", Title: "Apply Command", Kind: "command", Status: "draft", Body: "Applies pending plan to SQLite.", Hash: "h1"},
		{ID: "CMD-PLAN", Title: "Plan Command", Kind: "command", Status: "draft", Body: "Shows pending plan diff.", Hash: "h2"},
		{ID: "REQ-1", Title: "Import Requirement", Kind: "requirement", Status: "draft", Body: "Must import markdown files.", Hash: "h3"},
	})

	results, err := search.Search(database, "plan", search.Options{})
	require.NoError(t, err)
	assert.NotEmpty(t, results)
	// Both CMD-APPLY (body mentions "plan") and CMD-PLAN should appear.
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.Spec.ID)
	}
	assert.Contains(t, ids, "CMD-PLAN")
}

func TestSearch_KindFilter(t *testing.T) {
	database := openTestDB(t)
	seedSpecs(t, database, []db.Spec{
		{ID: "CMD-APPLY", Title: "Apply Command", Kind: "command", Status: "draft", Body: "Applies pending plan.", Hash: "h1"},
		{ID: "REQ-APPLY", Title: "Apply Requirement", Kind: "requirement", Status: "draft", Body: "System shall apply plan.", Hash: "h2"},
	})

	results, err := search.Search(database, "apply", search.Options{Kind: "requirement"})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	for _, r := range results {
		assert.Equal(t, "requirement", r.Spec.Kind)
	}
}

func TestSearch_StatusFilter(t *testing.T) {
	database := openTestDB(t)
	seedSpecs(t, database, []db.Spec{
		{ID: "REQ-1", Title: "Draft Apply", Kind: "requirement", Status: "draft", Body: "Draft apply spec.", Hash: "h1"},
		{ID: "REQ-2", Title: "Fixed Apply", Kind: "requirement", Status: "fixed", Body: "Fixed apply spec.", Hash: "h2"},
	})

	results, err := search.Search(database, "apply", search.Options{Status: "fixed"})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	for _, r := range results {
		assert.Equal(t, "fixed", r.Spec.Status)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	database := openTestDB(t)
	_, err := search.Search(database, "", search.Options{})
	assert.Error(t, err)
}

func TestSearch_NoResults(t *testing.T) {
	database := openTestDB(t)
	seedSpecs(t, database, []db.Spec{
		{ID: "REQ-1", Title: "Foo", Kind: "requirement", Status: "draft", Body: "Foo body.", Hash: "h1"},
	})

	results, err := search.Search(database, "xyzzy", search.Options{})
	require.NoError(t, err)
	assert.Empty(t, results)
}
