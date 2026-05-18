package normalizer_test

import (
	"testing"

	"github.com/speclite/speclite/internal/importer"
	"github.com/speclite/speclite/internal/normalizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ID assignment
// ---------------------------------------------------------------------------

func TestNormalize_UsesInlineID(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Import command."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "CMD-IMPORT", result[0].ID)
}

func TestNormalize_GeneratesSlugID(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "Import Command", InlineID: "", Body: "Import command."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	// Slug should be uppercase with hyphens.
	assert.Equal(t, "IMPORT-COMMAND", result[0].ID)
}

func TestNormalize_SlugFromSpecialChars(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "Plan/Apply Workflow!", InlineID: "", Body: ""},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "PLAN-APPLY-WORKFLOW", result[0].ID)
}

func TestNormalize_FallbackIDForEmptyTitle(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "", InlineID: "", Body: "first line is the title\nmore body."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	// ID should be generated from the first line of body.
	assert.NotEmpty(t, result[0].ID)
	assert.NotEqual(t, "SPEC-UNKNOWN", result[0].ID)
}

func TestNormalize_UnknownFallback(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "", InlineID: "", Body: ""},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "SPEC-UNKNOWN", result[0].ID)
}

// ---------------------------------------------------------------------------
// Duplicate ID disambiguation
// ---------------------------------------------------------------------------

func TestNormalize_DuplicateIDsDisambiguated(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "Overview", InlineID: "", Body: "first"},
		{Title: "Overview", InlineID: "", Body: "second"},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 2)
	assert.Equal(t, "OVERVIEW", result[0].ID)
	assert.Equal(t, "OVERVIEW-1", result[1].ID)
}

// ---------------------------------------------------------------------------
// Kind inference
// ---------------------------------------------------------------------------

func TestNormalize_KindCommand(t *testing.T) {
	cases := []struct {
		title string
		id    string
	}{
		{"CMD-IMPORT", "CMD-IMPORT"},
		{"Import command", ""},
		{"Command line interface", ""},
	}
	for _, tc := range cases {
		raws := []importer.RawSpec{{Title: tc.title, InlineID: tc.id, Body: ""}}
		result := normalizer.Normalize(raws)
		require.Len(t, result, 1, "title=%q", tc.title)
		assert.Equal(t, "command", result[0].Kind, "title=%q", tc.title)
	}
}

func TestNormalize_KindState(t *testing.T) {
	cases := []struct {
		title string
		id    string
	}{
		{"STATE-SQLITE", "STATE-SQLITE"},
		{"SQLite state store", ""},
	}
	for _, tc := range cases {
		raws := []importer.RawSpec{{Title: tc.title, InlineID: tc.id, Body: ""}}
		result := normalizer.Normalize(raws)
		require.Len(t, result, 1)
		assert.Equal(t, "state", result[0].Kind, "title=%q", tc.title)
	}
}

func TestNormalize_KindConstraint(t *testing.T) {
	cases := []struct {
		title string
		id    string
	}{
		{"CON-1", "CON-1"},
		{"CONSTRAINT-2", "CONSTRAINT-2"},
		{"Constraint: ID uniqueness", ""},
	}
	for _, tc := range cases {
		raws := []importer.RawSpec{{Title: tc.title, InlineID: tc.id, Body: ""}}
		result := normalizer.Normalize(raws)
		require.Len(t, result, 1)
		assert.Equal(t, "constraint", result[0].Kind, "title=%q", tc.title)
	}
}

func TestNormalize_KindRequirement(t *testing.T) {
	cases := []struct {
		title string
		id    string
	}{
		{"FR-1", "FR-1"},
		{"REQ-42", "REQ-42"},
		{"The system shall export Markdown", ""},
		{"This must be supported", ""},
	}
	for _, tc := range cases {
		raws := []importer.RawSpec{{Title: tc.title, InlineID: tc.id, Body: ""}}
		result := normalizer.Normalize(raws)
		require.Len(t, result, 1)
		assert.Equal(t, "requirement", result[0].Kind, "title=%q", tc.title)
	}
}

func TestNormalize_KindDefaultsToRequirement(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "General Overview", InlineID: "", Body: "Generic description."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "requirement", result[0].Kind)
}

// ---------------------------------------------------------------------------
// Status default
// ---------------------------------------------------------------------------

func TestNormalize_StatusDefaultsDraft(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Body."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "draft", result[0].Status)
}

// ---------------------------------------------------------------------------
// Hash
// ---------------------------------------------------------------------------

func TestNormalize_HashIsSet(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Body."},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Len(t, result[0].Hash, 64, "SHA-256 hex should be 64 chars")
}

func TestNormalize_HashIsDeterministic(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Body."},
	}
	r1 := normalizer.Normalize(raws)
	r2 := normalizer.Normalize(raws)
	assert.Equal(t, r1[0].Hash, r2[0].Hash)
}

func TestNormalize_HashDiffersWhenBodyChanges(t *testing.T) {
	r1 := normalizer.Normalize([]importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Body A."},
	})
	r2 := normalizer.Normalize([]importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "Body B."},
	})
	assert.NotEqual(t, r1[0].Hash, r2[0].Hash)
}

// ---------------------------------------------------------------------------
// Relations are preserved
// ---------------------------------------------------------------------------

func TestNormalize_RelationsPreserved(t *testing.T) {
	raws := []importer.RawSpec{
		{
			Title:    "CMD-APPLY",
			InlineID: "CMD-APPLY",
			Body:     "Apply the plan.",
			Relations: []importer.RelationHint{
				{Kind: "depends_on", TargetID: "CMD-PLAN"},
			},
		},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	require.Len(t, result[0].Relations, 1)
	assert.Equal(t, "depends_on", result[0].Relations[0].Kind)
	assert.Equal(t, "CMD-PLAN", result[0].Relations[0].TargetID)
}

// ---------------------------------------------------------------------------
// Body trimming
// ---------------------------------------------------------------------------

func TestNormalize_BodyTrimmed(t *testing.T) {
	raws := []importer.RawSpec{
		{Title: "CMD-IMPORT", InlineID: "CMD-IMPORT", Body: "  \n  Body content.  \n  "},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.Equal(t, "Body content.", result[0].Body)
}

// ---------------------------------------------------------------------------
// Slug truncation
// ---------------------------------------------------------------------------

func TestNormalize_SlugTruncated(t *testing.T) {
	longTitle := "This Is An Extremely Long Specification Title That Exceeds The Maximum Slug Length"
	raws := []importer.RawSpec{
		{Title: longTitle, InlineID: "", Body: ""},
	}
	result := normalizer.Normalize(raws)
	require.Len(t, result, 1)
	assert.LessOrEqual(t, len(result[0].ID), 40)
}
