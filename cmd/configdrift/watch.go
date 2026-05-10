package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/watcher"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for config drift continuously",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Handle graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		w, err := watcher.FromPolicy(ctx, cfg)
		if err != nil {
			return fmt.Errorf("failed to create watcher: %w", err)
		}

		if err := w.Run(ctx); err != nil && ctx.Err() == nil {
			return fmt.Errorf("watcher failed: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(watchCmd)
}
