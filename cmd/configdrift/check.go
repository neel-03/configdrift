package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/watcher"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run a single drift check",
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		w, err := watcher.FromPolicy(ctx, cfg, nil)
		if err != nil {
			return fmt.Errorf("failed to create watcher: %w", err)
		}

		results, err := w.RunCycle(ctx)
		if err != nil {
			return fmt.Errorf("check failed: %w", err)
		}

		drifted := false
		for _, res := range results {
			if res.IsDrifted {
				drifted = true
				slog.Warn("Drift detected", "target", res.TargetName)
				for _, d := range res.Added {
					slog.Warn("ADDED", "key", d.Key, "value", d.TargetValue)
				}
				for _, d := range res.Removed {
					slog.Warn("REMOVED", "key", d.Key, "value", d.CanonicalValue)
				}
				for _, d := range res.Changed {
					slog.Warn("CHANGED", "key", d.Key, "from", d.CanonicalValue, "to", d.TargetValue)
				}
			} else {
				slog.Info("No drift detected", "target", res.TargetName)
			}
		}

		if drifted {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
