// Package main is the entry point for the speqlite CLI.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "speqlite",
	Short: "Speclite — local-first specification state management CLI",
	Long: `Speclite treats specifications as state (like Terraform's plan/apply workflow).

Canonical state is stored in SQLite. Markdown is a projection for human editing.

Workflow:
  speqlite init                      Initialise a new workspace
  speqlite import FILE               Parse specs from Markdown/text (produces a plan)
  speqlite plan                      Show pending plan
  speqlite apply                     Apply pending plan to SQLite state
  speqlite render --all              Re-generate Markdown from state
  speqlite render [--format] ID      Render a single spec
  speqlite search "QUERY"            Full-text search (FTS5 BM25)
  speqlite deps ID                   Print dependency graph for a spec
  speqlite validate                  Check for structural issues
  speqlite state list                List all specs in state
  speqlite state show ID             Show full spec details
  speqlite state export              Export state snapshot as JSON
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
		newVersionCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
