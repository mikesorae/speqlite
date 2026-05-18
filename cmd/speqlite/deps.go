package main

import (
	"fmt"
	"os"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/workspace"
	"github.com/spf13/cobra"
)

func newDepsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deps <ID>",
		Short: "Print the dependency graph for a spec",
		Long: `Prints all outgoing depends_on relations for the given spec as a tree.

Example:
  speqlite deps CMD-APPLY

Output:
  CMD-APPLY
   ├── CMD-PLAN
   └── STATE-SQLITE`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("deps: get working directory: %w", err)
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("deps: %w", err)
			}

			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("deps: open database: %w", err)
			}
			defer database.Close()

			// Verify the root spec exists.
			if _, err := database.GetSpec(specID); err != nil {
				return fmt.Errorf("deps: spec %q not found", specID)
			}

			fmt.Println(specID)
			visited := map[string]bool{specID: true}
			if err := printDepsTree(database, specID, "", visited); err != nil {
				return fmt.Errorf("deps: %w", err)
			}

			return nil
		},
	}

	return cmd
}

// printDepsTree recursively prints depends_on relations as a tree.
// prefix is the leading whitespace + tree characters for the current level.
func printDepsTree(database *db.DB, specID, prefix string, visited map[string]bool) error {
	rels, err := database.ListRelationsFrom(specID)
	if err != nil {
		return fmt.Errorf("list relations from %q: %w", specID, err)
	}

	// Filter to depends_on relations only.
	var deps []db.Relation
	for _, r := range rels {
		if r.Relation == "depends_on" {
			deps = append(deps, r)
		}
	}

	for i, dep := range deps {
		isLast := i == len(deps)-1

		var connector, childPrefix string
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		} else {
			connector = "├── "
			childPrefix = prefix + "│   "
		}

		// Check for already-visited nodes to avoid infinite loops on cycles.
		if visited[dep.ToID] {
			fmt.Printf("%s%s%s (cycle)\n", prefix, connector, dep.ToID)
			continue
		}

		fmt.Printf("%s%s%s\n", prefix, connector, dep.ToID)
		visited[dep.ToID] = true
		if err := printDepsTree(database, dep.ToID, childPrefix, visited); err != nil {
			return err
		}
	}

	return nil
}
