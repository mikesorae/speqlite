package main

import (
	"fmt"
	"os"

	"github.com/speclite/speclite/internal/apply"
	"github.com/speclite/speclite/internal/db"
	"github.com/speclite/speclite/internal/planner"
	"github.com/speclite/speclite/internal/workspace"
	"github.com/spf13/cobra"
)

func newApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending plan to SQLite state",
		Long: `Reads .spec/state.plan.json and applies all pending changes to SQLite.

Each change is recorded in the event_log. A new snapshot is written to
.spec/state.snapshot.json after all changes are applied.

Run 'speclite plan' to review pending changes before applying.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("apply: get working directory: %w", err)
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("apply: %w", err)
			}

			// Load the pending plan.
			plan, err := planner.Load(ws.PlanPath)
			if err != nil {
				return fmt.Errorf("apply: load plan: %w", err)
			}

			if plan.IsEmpty() {
				fmt.Println("No pending changes to apply.")
				return nil
			}

			creates, updates, deletes := plan.Summary()
			fmt.Printf("Applying plan: %d to create, %d to update, %d to delete...\n",
				creates, updates, deletes)

			// Open database.
			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("apply: open database: %w", err)
			}
			defer database.Close()

			// Apply the plan.
			result, err := apply.Apply(database, plan, ws.SnapshotPath)
			if err != nil {
				return fmt.Errorf("apply: %w", err)
			}

			fmt.Printf("Applied: %d created, %d updated, %d deleted.\n",
				result.Created, result.Updated, result.Deleted)
			fmt.Printf("Snapshot written to %s\n", ws.SnapshotPath)

			// Remove the plan file to signal it has been applied.
			if err := os.Remove(ws.PlanPath); err != nil && !os.IsNotExist(err) {
				// Non-fatal: just warn.
				fmt.Fprintf(os.Stderr, "warning: could not remove plan file: %v\n", err)
			}

			return nil
		},
	}

	return cmd
}
