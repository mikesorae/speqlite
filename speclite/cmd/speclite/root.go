// Package main is the entry point for the speclite CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "speclite",
	Short: "Speclite — local-first specification state management CLI",
	Long: `Speclite treats specifications as state (like Terraform's plan/apply workflow).

Canonical state is stored in SQLite. Markdown is a projection for human editing.

Workflow:
  speclite init                      Initialise a new workspace
  speclite import FILE               Parse specs from Markdown/text (produces a plan)
  speclite plan                      Show pending plan
  speclite apply                     Apply pending plan to SQLite state
  speclite render --all              Re-generate Markdown from state
  speclite render [--format] ID      Render a single spec
  speclite search "QUERY"            Full-text search (FTS5 BM25)
  speclite deps ID                   Print dependency graph for a spec
  speclite validate                  Check for structural issues
  speclite state list                List all specs in state
  speclite state show ID             Show full spec details
  speclite state export              Export state snapshot as JSON
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	rootCmd.AddCommand(
		newInitCmd(),
		newImportCmd(),
		newPlanCmd(),
		newApplyCmd(),
		newRenderCmd(),
		newSearchCmd(),
		newDepsCmd(),
		newValidateCmd(),
		newStateCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
