package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number of configdrift",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("configdrift %s (commit: %s, built at: %s)\n", version.Version, version.Commit, version.BuildTime)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
