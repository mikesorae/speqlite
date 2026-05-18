package importer_test

import (
	"testing"

	"github.com/mikesorae/speqlite/internal/importer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// ParseFile — Markdown with headings
// ---------------------------------------------------------------------------

func TestParseFile_MarkdownHeadings(t *testing.T) {
	src := []byte(`# CMD-IMPORT

Import command specification. This command reads a Markdown file.

# CMD-APPLY

Apply command. depends_on: CMD-IMPORT
`)
	result, err := importer.ParseFile("test.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 2)

	s0 := result.Specs[0]
	assert.Equal(t, "CMD-IMPORT", s0.Title)
	assert.Equal(t, "CMD-IMPORT", s0.InlineID)

	s1 := result.Specs[1]
	assert.Equal(t, "CMD-APPLY", s1.Title)
	assert.Equal(t, "CMD-APPLY", s1.InlineID)
	require.Len(t, s1.Relations, 1)
	assert.Equal(t, "depends_on", s1.Relations[0].Kind)
	assert.Equal(t, "CMD-IMPORT", s1.Relations[0].TargetID)
}

func TestParseFile_MarkdownInlineID(t *testing.T) {
	src := []byte(`# Import Specification

FR-1 This requirement covers the import functionality.
`)
	result, err := importer.ParseFile("test.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 1)

	s := result.Specs[0]
	assert.Equal(t, "Import Specification", s.Title)
	assert.Equal(t, "FR-1", s.InlineID)
}

func TestParseFile_MultipleHeadingLevels(t *testing.T) {
	src := []byte(`# FR-1 Import

Description of FR-1.

## FR-1-1 Sub-requirement

Sub-requirement detail.

# FR-2 Plan

Plan requirement.
`)
	result, err := importer.ParseFile("test.md", src)
	require.NoError(t, err)
	// Each heading creates a spec candidate.
	assert.GreaterOrEqual(t, len(result.Specs), 2)
}

func TestParseFile_MarkdownRelationHints(t *testing.T) {
	src := []byte(`# STATE-SQLITE

SQLite state store.

implements: CMD-APPLY
related_to: CMD-PLAN
`)
	result, err := importer.ParseFile("state.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 1)

	s := result.Specs[0]
	assert.Equal(t, "STATE-SQLITE", s.InlineID)
	require.Len(t, s.Relations, 2)

	relKinds := map[string]string{}
	for _, r := range s.Relations {
		relKinds[r.Kind] = r.TargetID
	}
	assert.Equal(t, "CMD-APPLY", relKinds["implements"])
	assert.Equal(t, "CMD-PLAN", relKinds["related_to"])
}

func TestParseFile_EmptyFile(t *testing.T) {
	result, err := importer.ParseFile("empty.md", []byte(""))
	require.NoError(t, err)
	assert.Empty(t, result.Specs)
}

func TestParseFile_WhitespaceOnly(t *testing.T) {
	result, err := importer.ParseFile("ws.md", []byte("   \n\n   \n"))
	require.NoError(t, err)
	assert.Empty(t, result.Specs)
}

// ---------------------------------------------------------------------------
// ParseFile — Plain text (no headings)
// ---------------------------------------------------------------------------

func TestParseFile_PlainText(t *testing.T) {
	src := []byte(`CMD-RENDER render command

Renders markdown from state.

CMD-PLAN plan command

Shows the pending plan.
`)
	result, err := importer.ParseFile("scratch.txt", src)
	require.NoError(t, err)
	// Plain text falls back to paragraph splitting.
	assert.GreaterOrEqual(t, len(result.Specs), 1)
}

func TestParseFile_PlainTextRelations(t *testing.T) {
	src := []byte(`CMD-APPLY apply command

Applies the plan. depends_on: CMD-PLAN
`)
	result, err := importer.ParseFile("scratch.txt", src)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result.Specs), 1)

	// At least one spec should carry the relation hint.
	var found bool
	for _, s := range result.Specs {
		for _, r := range s.Relations {
			if r.Kind == "depends_on" && r.TargetID == "CMD-PLAN" {
				found = true
			}
		}
	}
	assert.True(t, found, "expected depends_on: CMD-PLAN relation hint")
}

// ---------------------------------------------------------------------------
// Inline ID extraction edge cases
// ---------------------------------------------------------------------------

func TestParseFile_NoInlineID(t *testing.T) {
	src := []byte(`# Overview

General overview without an explicit spec ID.
`)
	result, err := importer.ParseFile("overview.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 1)
	assert.Equal(t, "", result.Specs[0].InlineID)
}

func TestParseFile_SourceFileIsSet(t *testing.T) {
	src := []byte("# Title\n\nBody.")
	result, err := importer.ParseFile("myfile.md", src)
	require.NoError(t, err)
	assert.Equal(t, "myfile.md", result.SourceFile)
}

// ---------------------------------------------------------------------------
// Relation deduplication
// ---------------------------------------------------------------------------

func TestParseFile_RelationDeduplication(t *testing.T) {
	src := []byte(`# CMD-EXPORT

depends_on: CMD-PLAN
depends_on: CMD-PLAN
`)
	result, err := importer.ParseFile("test.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 1)
	// Duplicate relation should be deduplicated.
	assert.Len(t, result.Specs[0].Relations, 1)
}

// ---------------------------------------------------------------------------
// Various relation keyword forms
// ---------------------------------------------------------------------------

func TestParseFile_RelationKeywordVariants(t *testing.T) {
	src := []byte(`# SPEC-A

depends on CMD-B
implements CMD-C
verifies CMD-D
supersedes CMD-E
related to CMD-F
`)
	result, err := importer.ParseFile("test.md", src)
	require.NoError(t, err)
	require.Len(t, result.Specs, 1)

	relMap := map[string]string{}
	for _, r := range result.Specs[0].Relations {
		relMap[r.Kind] = r.TargetID
	}
	assert.Equal(t, "CMD-B", relMap["depends_on"])
	assert.Equal(t, "CMD-C", relMap["implements"])
	assert.Equal(t, "CMD-D", relMap["verifies"])
	assert.Equal(t, "CMD-E", relMap["supersedes"])
	assert.Equal(t, "CMD-F", relMap["related_to"])
}
