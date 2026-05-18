package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Build-time variables injected via -ldflags.
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Long:  "Print the version, commit hash, and build date of this speqlite binary.",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("speqlite %s (commit: %s, built: %s)\n", Version, Commit, BuildDate)
		},
	}
}
