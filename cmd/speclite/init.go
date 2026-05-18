package main

import (
	"fmt"
	"os"

	"github.com/speclite/speclite/internal/db"
	"github.com/speclite/speclite/internal/workspace"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialise a new Speclite workspace in the current directory",
		Long: `Creates the .spec/ directory, initialises the SQLite database,
and creates the specs/, scratch/, and changes/ directories.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("init: get working directory: %w", err)
			}

			if err := workspace.Init(cwd, force); err != nil {
				return err
			}

			ws, err := workspace.Find(cwd)
			if err != nil {
				return fmt.Errorf("init: find workspace after creation: %w", err)
			}

			database, err := db.Open(ws.DBPath)
			if err != nil {
				return fmt.Errorf("init: open database: %w", err)
			}
			defer database.Close()

			if err := database.Init(); err != nil {
				return fmt.Errorf("init: record init event: %w", err)
			}

			fmt.Printf("Initialised Speclite workspace at %s\n", ws.Root)
			fmt.Printf("  Database: %s\n", ws.DBPath)
			fmt.Printf("  Specs:    %s\n", ws.SpecsDir)
			fmt.Printf("  Scratch:  %s\n", ws.ScratchDir)
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Reinitialise an existing workspace")
	return cmd
}
