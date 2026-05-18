package main

import (
	"fmt"
	"os"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/renderer"
	"github.com/mikesorae/speqlite/internal/workspace"
	"github.com/spf13/cobra"
)

func newRenderCmd() *cobra.Command {
	var (
		renderAll    bool
		formatStr    string
	)

	cmd := &cobra.Command{
		Use:   "render [--all] [--format markdown|text|json] [ID]",
		Short: "Render specs from SQLite to files in specs/",
		Long: `Renders one or all specs from SQLite state to files in specs/.

With --all, iterates every non-deprecated spec. Without --all, a spec ID
is required as the first argument.

Output format defaults to markdown. Files are written as specs/<ID>.<ext>.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := renderer.ParseFormat(formatStr)
			if err != nil {
				return fmt.Errorf("render: %w", err)
			}

			if !renderAll && len(args) == 0 {
				return fmt.Errorf("render: provide a spec ID or use --all")
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("render: get working directory: %w", err)
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("render: %w", err)
			}

			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("render: open database: %w", err)
			}
			defer database.Close()

			// Ensure specs/ directory exists.
			if err := os.MkdirAll(ws.SpecsDir, 0o755); err != nil {
				return fmt.Errorf("render: create specs dir: %w", err)
			}

			if renderAll {
				return renderAll_(database, ws.SpecsDir, format)
			}

			specID := args[0]
			return renderOne(database, specID, ws.SpecsDir, format)
		},
	}

	cmd.Flags().BoolVar(&renderAll, "all", false, "Render all non-deprecated specs")
	cmd.Flags().StringVar(&formatStr, "format", "markdown", "Output format: markdown, text, json")

	return cmd
}

func renderOne(database *db.DB, specID, specsDir string, format renderer.Format) error {
	spec, err := database.GetSpec(specID)
	if err != nil {
		return fmt.Errorf("render: get spec %q: %w", specID, err)
	}

	rels, err := database.ListRelationsFrom(specID)
	if err != nil {
		return fmt.Errorf("render: list relations for %q: %w", specID, err)
	}

	path, err := renderer.WriteSpecFile(spec, rels, specsDir, format)
	if err != nil {
		return fmt.Errorf("render: write spec file: %w", err)
	}

	fmt.Printf("Rendered %s → %s\n", specID, path)
	return nil
}

func renderAll_(database *db.DB, specsDir string, format renderer.Format) error {
	specs, err := database.ListSpecs("", "")
	if err != nil {
		return fmt.Errorf("render: list specs: %w", err)
	}

	count := 0
	for _, spec := range specs {
		if spec.Status == "deprecated" {
			continue
		}

		rels, err := database.ListRelationsFrom(spec.ID)
		if err != nil {
			return fmt.Errorf("render: list relations for %q: %w", spec.ID, err)
		}

		path, err := renderer.WriteSpecFile(spec, rels, specsDir, format)
		if err != nil {
			return fmt.Errorf("render: write spec file %q: %w", spec.ID, err)
		}

		fmt.Printf("  %s → %s\n", spec.ID, path)
		count++
	}

	fmt.Printf("\nRendered %d spec(s) to %s\n", count, specsDir)
	return nil
}
