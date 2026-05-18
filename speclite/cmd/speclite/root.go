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
  speclite init           Initialise a new workspace
  speclite import FILE    Parse specs from Markdown/text (produces a plan)
  speclite plan           Show pending plan
  speclite apply          Apply pending plan to SQLite state
  speclite render --all   Re-generate Markdown from state
  speclite search QUERY   Full-text search (FTS5 BM25)
  speclite validate       Check for structural issues
  speclite state list     List all specs
`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func main() {
	rootCmd.AddCommand(
		newInitCmd(),
	)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
