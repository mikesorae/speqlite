// Package workspace handles workspace discovery and initialisation.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	specDir      = ".spec"
	dbFileName   = "state.sqlite"
	planFileName = "state.plan.json"
	snapFileName = "state.snapshot.json"
	specsDir     = "specs"
	scratchDir   = "scratch"
	changesDir   = "changes"
)

// Workspace holds all resolved paths for a Speclite workspace.
type Workspace struct {
	Root         string // directory containing .spec/
	DBPath       string // .spec/state.sqlite
	PlanPath     string // .spec/state.plan.json
	SnapshotPath string // .spec/state.snapshot.json
	SpecsDir     string // specs/
	ScratchDir   string // scratch/
	ChangesDir   string // changes/
}

// ErrNotFound is returned when no .spec/ directory is found in the directory tree.
var ErrNotFound = errors.New("workspace: .spec/ directory not found; run 'speclite init' to create one")

// Find walks up the directory tree from start, looking for a .spec/ directory.
// It returns the first Workspace found, or ErrNotFound.
func Find(start string) (*Workspace, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, fmt.Errorf("workspace: resolve start path: %w", err)
	}

	for {
		candidate := filepath.Join(dir, specDir)
		info, err := os.Stat(candidate)
		if err == nil && info.IsDir() {
			return buildWorkspace(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root.
			break
		}
		dir = parent
	}

	return nil, ErrNotFound
}

// Init creates a new Speclite workspace at root.
// If force is false and the workspace already exists, it returns an error.
func Init(root string, force bool) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("workspace: resolve root: %w", err)
	}

	specPath := filepath.Join(absRoot, specDir)
	info, statErr := os.Stat(specPath)
	if statErr == nil && info.IsDir() {
		if !force {
			return fmt.Errorf("workspace: already initialised at %s", absRoot)
		}
	}

	// Create required directories.
	dirs := []string{
		specPath,
		filepath.Join(absRoot, specsDir),
		filepath.Join(absRoot, scratchDir),
		filepath.Join(absRoot, changesDir),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("workspace: create dir %q: %w", d, err)
		}
	}

	return nil
}

// buildWorkspace constructs a Workspace from a resolved root path.
func buildWorkspace(root string) *Workspace {
	return &Workspace{
		Root:         root,
		DBPath:       filepath.Join(root, specDir, dbFileName),
		PlanPath:     filepath.Join(root, specDir, planFileName),
		SnapshotPath: filepath.Join(root, specDir, snapFileName),
		SpecsDir:     filepath.Join(root, specsDir),
		ScratchDir:   filepath.Join(root, scratchDir),
		ChangesDir:   filepath.Join(root, changesDir),
	}
}
