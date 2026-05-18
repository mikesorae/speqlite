package renderer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/renderer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleSpec() db.Spec {
	return db.Spec{
		ID:        "CMD-RENDER",
		Title:     "Render Command",
		Kind:      "command",
		Status:    "draft",
		Version:   1,
		Body:      "Renders specs from SQLite to Markdown.",
		Hash:      "abc123",
		CreatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
	}
}

func sampleRels() []db.Relation {
	return []db.Relation{
		{FromID: "CMD-RENDER", Relation: "depends_on", ToID: "STATE-SQLITE"},
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    renderer.Format
		wantErr bool
	}{
		{"markdown", renderer.FormatMarkdown, false},
		{"md", renderer.FormatMarkdown, false},
		{"text", renderer.FormatText, false},
		{"txt", renderer.FormatText, false},
		{"plain", renderer.FormatText, false},
		{"json", renderer.FormatJSON, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := renderer.ParseFormat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRenderSpec_Markdown(t *testing.T) {
	s := sampleSpec()
	rels := sampleRels()

	out, err := renderer.RenderSpec(s, rels, renderer.FormatMarkdown)
	require.NoError(t, err)

	assert.Contains(t, out, "# Render Command")
	assert.Contains(t, out, "CMD-RENDER")
	assert.Contains(t, out, "command")
	assert.Contains(t, out, "draft")
	assert.Contains(t, out, "depends_on")
	assert.Contains(t, out, "STATE-SQLITE")
	assert.Contains(t, out, "Renders specs from SQLite")
}

func TestRenderSpec_Text(t *testing.T) {
	s := sampleSpec()
	rels := sampleRels()

	out, err := renderer.RenderSpec(s, rels, renderer.FormatText)
	require.NoError(t, err)

	assert.Contains(t, out, "Render Command")
	assert.Contains(t, out, "CMD-RENDER")
	assert.Contains(t, out, "command")
	assert.Contains(t, out, "draft")
	assert.Contains(t, out, "depends_on")
	assert.True(t, strings.HasPrefix(out, "Render Command\n"))
}

func TestRenderSpec_JSON(t *testing.T) {
	s := sampleSpec()
	rels := sampleRels()

	out, err := renderer.RenderSpec(s, rels, renderer.FormatJSON)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))

	assert.Equal(t, "CMD-RENDER", parsed["id"])
	assert.Equal(t, "Render Command", parsed["title"])
	assert.Equal(t, "command", parsed["kind"])
	assert.Equal(t, "draft", parsed["status"])
}

func TestRenderSpec_NoRelations(t *testing.T) {
	s := sampleSpec()

	out, err := renderer.RenderSpec(s, nil, renderer.FormatMarkdown)
	require.NoError(t, err)
	assert.NotContains(t, out, "## Relations")
}

func TestWriteSpecFile_Markdown(t *testing.T) {
	dir := t.TempDir()
	s := sampleSpec()
	rels := sampleRels()

	path, err := renderer.WriteSpecFile(s, rels, dir, renderer.FormatMarkdown)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "CMD-RENDER.md"), path)

	_, err = os.Stat(path)
	assert.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# Render Command")
}

func TestWriteSpecFile_JSON(t *testing.T) {
	dir := t.TempDir()
	s := sampleSpec()

	path, err := renderer.WriteSpecFile(s, nil, dir, renderer.FormatJSON)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "CMD-RENDER.json"), path)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(content, &parsed))
	assert.Equal(t, "CMD-RENDER", parsed["id"])
}
