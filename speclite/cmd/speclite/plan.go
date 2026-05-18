package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/speclite/speclite/internal/db"
	"github.com/speclite/speclite/internal/planner"
	"github.com/speclite/speclite/internal/workspace"
	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Re-run the planner against the current snapshot and pretty-print the diff",
		Long: `Loads the pending plan from .spec/state.plan.json and pretty-prints it.

If no plan exists yet, run 'speclite import <file>' first.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("plan: get working directory: %w", err)
			}
			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}

			// Load the existing plan file.
			plan, err := planner.Load(ws.PlanPath)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					fmt.Println("No pending plan. Run 'speclite import <file>' to create one.")
					return nil
				}
				return fmt.Errorf("plan: load plan: %w", err)
			}

			if plan.IsEmpty() {
				fmt.Println("Plan: no changes.")
				return nil
			}

			// Open database to resolve current state for context.
			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("plan: open database: %w", err)
			}
			defer database.Close()

			printPlanDiff(plan)
			return nil
		},
	}

	return cmd
}

// printPlanDiff pretty-prints the full plan diff with before/after details.
func printPlanDiff(plan *planner.Plan) {
	creates, updates, deletes := plan.Summary()

	fmt.Printf("Pending plan (generated %s)\n\n", plan.CreatedAt.Format("2006-01-02 15:04:05 UTC"))

	for _, entry := range plan.Entries {
		switch entry.Action {
		case planner.ActionCreate:
			fmt.Printf("+ %s  [create]\n", entry.SpecID)
			if entry.After != nil {
				fmt.Printf("    title:  %s\n", entry.After.Title)
				fmt.Printf("    kind:   %s\n", entry.After.Kind)
				fmt.Printf("    status: %s\n", entry.After.Status)
			}

		case planner.ActionUpdate:
			fmt.Printf("~ %s  [update]\n", entry.SpecID)
			if entry.Before != nil && entry.After != nil {
				if entry.Before.Title != entry.After.Title {
					fmt.Printf("    title:  %q → %q\n", entry.Before.Title, entry.After.Title)
				}
				if entry.Before.Kind != entry.After.Kind {
					fmt.Printf("    kind:   %q → %q\n", entry.Before.Kind, entry.After.Kind)
				}
				if entry.Before.Status != entry.After.Status {
					fmt.Printf("    status: %q → %q\n", entry.Before.Status, entry.After.Status)
				}
				if entry.Before.Hash != entry.After.Hash {
					fmt.Printf("    hash:   %s → %s\n", entry.Before.Hash[:8], entry.After.Hash[:8])
				}
			}

		case planner.ActionDelete:
			fmt.Printf("- %s  [delete]\n", entry.SpecID)
			if entry.Before != nil {
				fmt.Printf("    title:  %s\n", entry.Before.Title)
				fmt.Printf("    kind:   %s\n", entry.Before.Kind)
				fmt.Printf("    status: %s\n", entry.Before.Status)
			}
		}
	}

	fmt.Printf("\n%d to create, %d to update, %d to delete.\n", creates, updates, deletes)
	fmt.Println("Run 'speclite apply' to apply this plan.")
}
