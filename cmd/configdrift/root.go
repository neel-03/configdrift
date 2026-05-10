package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/neel-03/configdrift/internal/logger"
)

var (
	cfgFile     string
	logLevel    string
	closeLogger func()
)

var rootCmd = &cobra.Command{
	Use:   "configdrift",
	Short: "Detect config drift across your infrastructure",
	PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
		_ = godotenv.Load() // ignore error if .env doesn't exist
		var err error
		closeLogger, err = logger.Init(logLevel)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		return nil
	},
}

func Execute() {
	err := rootCmd.Execute()
	if closeLogger != nil {
		closeLogger()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "source.yaml", "config file (default is source.yaml)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}
