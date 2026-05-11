package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/daemon"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the configdrift management daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := daemon.NewClient("")
		if client.IsRunning() {
			return fmt.Errorf("daemon is already running on %s", daemon.DefaultSocketPath)
		}

		srv := daemon.NewServer("")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("Starting configdrift daemon on %s...\n", daemon.DefaultSocketPath)
		if err := srv.Start(ctx); err != nil {
			return fmt.Errorf("daemon failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
