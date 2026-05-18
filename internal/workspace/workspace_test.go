package workspace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	root := t.TempDir()

	err := Init(root, false)
	require.NoError(t, err)

	// Check expected directories exist.
	for _, d := range []string{
		filepath.Join(root, specDir),
		filepath.Join(root, specsDir),
		filepath.Join(root, scratchDir),
		filepath.Join(root, changesDir),
	} {
		info, statErr := os.Stat(d)
		require.NoError(t, statErr, "directory %s should exist", d)
		assert.True(t, info.IsDir(), "%s should be a directory", d)
	}
}

func TestInit_AlreadyInitialised(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Init(root, false))

	// Second init without force should fail.
	err := Init(root, false)
	assert.Error(t, err, "reinitialising without --force should error")
}

func TestInit_Force(t *testing.T) {
	root := t.TempDir()

	require.NoError(t, Init(root, false))
	// Second init with force should succeed.
	require.NoError(t, Init(root, true))
}

func TestFind(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Init(root, false))

	// Find from root itself.
	ws, err := Find(root)
	require.NoError(t, err)
	assert.Equal(t, root, ws.Root)
	assert.Equal(t, filepath.Join(root, specDir, dbFileName), ws.DBPath)
	assert.Equal(t, filepath.Join(root, specDir, planFileName), ws.PlanPath)
	assert.Equal(t, filepath.Join(root, specDir, snapFileName), ws.SnapshotPath)
}

func TestFind_FromSubdirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, Init(root, false))

	// Create a nested subdirectory and find from there.
	subdir := filepath.Join(root, "a", "b", "c")
	require.NoError(t, os.MkdirAll(subdir, 0o755))

	ws, err := Find(subdir)
	require.NoError(t, err)
	assert.Equal(t, root, ws.Root)
}

func TestFind_NotFound(t *testing.T) {
	// Create an isolated directory tree that has no .spec/ anywhere in its ancestry.
	// We must avoid using t.TempDir() directly if /tmp already has a .spec/ from
	// a previous run. Instead, create a fresh root that we control.
	root := t.TempDir()

	// Remove any .spec that might exist in the root itself (shouldn't happen with TempDir,
	// but be defensive).
	os.RemoveAll(filepath.Join(root, specDir))

	// Walk up to verify our temp dir is not already inside a workspace.
	// If it is, skip the test with an explanation.
	_, parentErr := Find(filepath.Dir(root))
	if parentErr == nil {
		t.Skip("parent directory is already a Speclite workspace; cannot test ErrNotFound in this environment")
	}

	_, err := Find(root)
	assert.ErrorIs(t, err, ErrNotFound)
}
