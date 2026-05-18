package main

import (
	"fmt"
	"os"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/search"
	"github.com/mikesorae/speqlite/internal/workspace"
	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var (
		filterKind   string
		filterStatus string
	)

	cmd := &cobra.Command{
		Use:   `search "<query>" [--type <kind>] [--status <status>]`,
		Short: "Full-text search over specs using FTS5 BM25 ranking",
		Long: `Searches spec titles and bodies using SQLite FTS5 with BM25 ranking.

Results are printed best-match-first (lowest BM25 score = most relevant).

Examples:
  speqlite search "plan apply"
  speqlite search "import" --type command
  speqlite search "requirement" --status fixed`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("search: get working directory: %w", err)
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("search: open database: %w", err)
			}
			defer database.Close()

			results, err := search.Search(database, query, search.Options{
				Kind:   filterKind,
				Status: filterStatus,
			})
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No results found.")
				return nil
			}

			fmt.Printf("%-20s %-14s %-12s %s\n", "ID", "KIND", "STATUS", "TITLE")
			fmt.Println("------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Printf("%-20s %-14s %-12s %s\n",
					r.Spec.ID, r.Spec.Kind, r.Spec.Status, r.Spec.Title)
			}
			fmt.Printf("\n%d result(s) for %q\n", len(results), query)
			return nil
		},
	}

	cmd.Flags().StringVar(&filterKind, "type", "", "Filter by spec kind (command, requirement, state, constraint)")
	cmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status (draft, review, fixed, implemented, verified, deprecated)")

	return cmd
}
