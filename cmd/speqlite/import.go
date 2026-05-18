package main

import (
	"fmt"
	"os"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/importer"
	"github.com/mikesorae/speqlite/internal/normalizer"
	"github.com/mikesorae/speqlite/internal/planner"
	"github.com/mikesorae/speqlite/internal/workspace"
	"github.com/spf13/cobra"
)

func newImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Parse specs from a Markdown or plain-text file and show the resulting plan",
		Long: `Parses spec candidates from a Markdown or plain-text file, normalises them,
and diffs the result against the current SQLite state.

The plan is written to .spec/state.plan.json but state is NOT mutated.
Run 'speqlite apply' to apply the plan.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]

			// Read source file.
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("import: read file %q: %w", filePath, err)
			}

			// Locate workspace.
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("import: get working directory: %w", err)
			}
			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}

			// Parse the file.
			result, err := importer.ParseFile(filePath, content)
			if err != nil {
				return fmt.Errorf("import: parse file: %w", err)
			}

			if len(result.Specs) == 0 {
				fmt.Println("No spec candidates found in file.")
				return nil
			}

			// Normalise.
			normalized := normalizer.Normalize(result.Specs)

			// Load current state from SQLite.
			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("import: open database: %w", err)
			}
			defer database.Close()

			current, err := database.ListSpecs("", "")
			if err != nil {
				return fmt.Errorf("import: list current specs: %w", err)
			}

			// Diff desired vs current.
			plan := planner.Diff(normalized, current)

			// Write plan to disk.
			if err := plan.Write(ws.PlanPath); err != nil {
				return fmt.Errorf("import: write plan: %w", err)
			}

			// Print summary.
			printPlanSummary(plan)
			return nil
		},
	}

	return cmd
}

// printPlanSummary prints a human-readable summary of the plan to stdout.
func printPlanSummary(plan *planner.Plan) {
	if plan.IsEmpty() {
		fmt.Println("Plan: no changes.")
		return
	}

	creates, updates, deletes := plan.Summary()
	fmt.Println("Plan:")
	for _, entry := range plan.Entries {
		switch entry.Action {
		case planner.ActionCreate:
			fmt.Printf("  + create %s\n", entry.SpecID)
		case planner.ActionUpdate:
			fmt.Printf("  ~ update %s\n", entry.SpecID)
		case planner.ActionDelete:
			fmt.Printf("  - delete %s\n", entry.SpecID)
		}
	}
	fmt.Printf("\n%d to create, %d to update, %d to delete.\n", creates, updates, deletes)
	fmt.Println("Run 'speqlite apply' to apply this plan.")
}
