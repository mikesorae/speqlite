package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mikesorae/speqlite/internal/db"
	"github.com/mikesorae/speqlite/internal/workspace"
	"github.com/spf13/cobra"
)

func newStateCmd() *cobra.Command {
	stateCmd := &cobra.Command{
		Use:   "state",
		Short: "Inspect or export the current spec state",
		Long:  `Commands for inspecting, querying, and exporting the canonical SQLite state.`,
	}

	stateCmd.AddCommand(
		newStateListCmd(),
		newStateShowCmd(),
		newStateExportCmd(),
	)

	return stateCmd
}

func newStateListCmd() *cobra.Command {
	var (
		filterKind   string
		filterStatus string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all specs in state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, ws, err := openDB()
			if err != nil {
				return fmt.Errorf("state list: %w", err)
			}
			defer database.Close()
			_ = ws

			specs, err := database.ListSpecs(filterKind, filterStatus)
			if err != nil {
				return fmt.Errorf("state list: %w", err)
			}

			if len(specs) == 0 {
				fmt.Println("No specs in state.")
				return nil
			}

			fmt.Printf("%-20s %-14s %-12s %3s  %s\n", "ID", "KIND", "STATUS", "VER", "TITLE")
			fmt.Println(strings.Repeat("-", 80))
			for _, s := range specs {
				fmt.Printf("%-20s %-14s %-12s %3d  %s\n",
					s.ID, s.Kind, s.Status, s.Version, s.Title)
			}
			fmt.Printf("\n%d spec(s) total\n", len(specs))
			return nil
		},
	}

	cmd.Flags().StringVar(&filterKind, "type", "", "Filter by kind")
	cmd.Flags().StringVar(&filterStatus, "status", "", "Filter by status")
	return cmd
}

func newStateShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <ID>",
		Short: "Show full details of a spec",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			specID := args[0]

			database, ws, err := openDB()
			if err != nil {
				return fmt.Errorf("state show: %w", err)
			}
			defer database.Close()
			_ = ws

			spec, err := database.GetSpec(specID)
			if err != nil {
				return fmt.Errorf("state show: %w", err)
			}

			rels, err := database.ListRelationsFrom(specID)
			if err != nil {
				return fmt.Errorf("state show: list relations: %w", err)
			}

			events, err := database.ListEvents(specID)
			if err != nil {
				return fmt.Errorf("state show: list events: %w", err)
			}

			fmt.Printf("ID:        %s\n", spec.ID)
			fmt.Printf("Title:     %s\n", spec.Title)
			fmt.Printf("Kind:      %s\n", spec.Kind)
			fmt.Printf("Status:    %s\n", spec.Status)
			fmt.Printf("Version:   %d\n", spec.Version)
			fmt.Printf("Hash:      %s\n", spec.Hash)
			fmt.Printf("Created:   %s\n", spec.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"))
			fmt.Printf("Updated:   %s\n", spec.UpdatedAt.UTC().Format("2006-01-02T15:04:05Z"))

			if spec.Body != "" {
				fmt.Printf("\nBody:\n%s\n", spec.Body)
			}

			if len(rels) > 0 {
				fmt.Println("\nRelations:")
				for _, r := range rels {
					fmt.Printf("  %s -[%s]-> %s\n", r.FromID, r.Relation, r.ToID)
				}
			}

			if len(events) > 0 {
				fmt.Println("\nEvent History:")
				for _, e := range events {
					fmt.Printf("  [%s] %s  %s\n",
						e.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
						e.EventType,
						e.PayloadJSON,
					)
				}
			}

			return nil
		},
	}
}

func newStateExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Export the current state snapshot as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			database, ws, err := openDB()
			if err != nil {
				return fmt.Errorf("state export: %w", err)
			}
			defer database.Close()

			snap, err := database.TakeSnapshot(ws.SnapshotPath)
			if err != nil {
				return fmt.Errorf("state export: %w", err)
			}

			data, err := json.MarshalIndent(snap, "", "  ")
			if err != nil {
				return fmt.Errorf("state export: marshal: %w", err)
			}

			fmt.Println(string(data))
			return nil
		},
	}
}

// openDB is a helper to locate the workspace and open the database.
func openDB() (*db.DB, *workspace.Workspace, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, nil, fmt.Errorf("get working directory: %w", err)
	}
	ws, err := workspace.Find(cwd)
	if err != nil {
		return nil, nil, err
	}
	database, err := db.Open(ws.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	return database, ws, nil
}
