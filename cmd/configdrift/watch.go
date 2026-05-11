package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/daemon"
	"github.com/neel-03/configdrift/internal/watcher"
)

var (
	detach bool
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch for config drift continuously",
	RunE: func(cmd *cobra.Command, args []string) error {
		if detach {
			client := daemon.NewClient("")
			if !client.IsRunning() {
				return fmt.Errorf("daemon is not running. Please run 'configdrift daemon' first")
			}

			resp, err := client.Send(daemon.Request{
				Type:   daemon.RequestStart,
				Config: cfgFile,
			})
			if err != nil {
				return err
			}

			if !resp.Success {
				return fmt.Errorf("failed to start background watcher: %s", resp.Message)
			}

			fmt.Println(resp.ID)
			return nil
		}

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Handle graceful shutdown for foreground mode
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		w, err := watcher.FromPolicy(ctx, cfg, nil)
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
	watchCmd.Flags().BoolVarP(&detach, "detach", "d", false, "run in background via daemon")
	rootCmd.AddCommand(watchCmd)
}
