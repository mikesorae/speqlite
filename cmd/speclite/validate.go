package main

import (
	"fmt"
	"os"

	"github.com/speclite/speclite/internal/db"
	"github.com/speclite/speclite/internal/validator"
	"github.com/speclite/speclite/internal/workspace"
	"github.com/spf13/cobra"
)

func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate structural integrity of the spec state",
		Long: `Runs structural validation checks against the SQLite state:

  • Relation integrity   — dangling references (to_id does not exist)
  • Invalid status       — unknown status values
  • Cyclic dependencies  — DFS on depends_on edges
  • Missing fields       — empty title, kind, status (errors); empty body (warning)

Exits with code 1 if any errors are found.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("validate: get working directory: %w", err)
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("validate: open database: %w", err)
			}
			defer database.Close()

			report, err := validator.Validate(database)
			if err != nil {
				return fmt.Errorf("validate: %w", err)
			}

			if len(report.Issues) == 0 {
				fmt.Println("Validation passed: no issues found.")
				return nil
			}

			for _, issue := range report.Issues {
				fmt.Println(issue.String())
			}

			fmt.Printf("\n%d error(s), %d warning(s)\n",
				report.ErrorCount(), report.WarningCount())

			if report.HasErrors() {
				return fmt.Errorf("validate: %d structural error(s) found", report.ErrorCount())
			}
			return nil
		},
	}

	return cmd
}
