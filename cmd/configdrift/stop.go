package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/daemon"
)

var stopCmd = &cobra.Command{
	Use:   "stop [ID]",
	Short: "Stop a background watcher",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient("")
		resp, err := client.Send(daemon.Request{
			Type: daemon.RequestStop,
			ID:   args[0],
		})
		if err != nil {
			return err
		}

		if !resp.Success {
			return fmt.Errorf("failed to stop watcher: %s", resp.Message)
		}

		fmt.Println(resp.Message)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
