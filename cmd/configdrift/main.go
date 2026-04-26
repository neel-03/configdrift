package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/diff"
	"github.com/neel-03/configdrift/internal/logger"
	"github.com/neel-03/configdrift/internal/parser"
	"github.com/neel-03/configdrift/internal/source"
	"github.com/neel-03/configdrift/internal/target"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// initializing the custom logger and getting cleanup function
	closeLogger, err := logger.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize logger: %w", err)
	}
	defer closeLogger()

	// load .env file if it exists
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	slog.Info("Configdrift starting")

	cfg, err := config.Load("./source.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	slog.Info("config loaded", "config", cfg)

	// create and fetch data from local config
	var src source.Source

	switch cfg.Canonical.Type {
	case config.TypeLocal:
		src = source.NewLocalSource(cfg.Canonical.Path)
	case config.TypeGit:
		src = source.NewGitSource(
			cfg.Canonical.Repo,
			cfg.Canonical.Branch,
			cfg.Canonical.Path,
			cfg.Canonical.AuthToken,
		)
	case config.TypeS3:
		src, err = source.NewS3Source(
			context.Background(),
			cfg.Canonical.S3Bucket,
			cfg.Canonical.Path,
			cfg.Canonical.S3Region,
		)
		if err != nil {
			return fmt.Errorf("failed to init s3 source: %w", err)
		}
	default:
		return fmt.Errorf("unsupported source type: %s", cfg.Canonical.Type)
	}

	// create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// fetch data from source
	data, err := src.Fetch(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	// create parser and parse the data
	p, err := parser.FromPath(cfg.Canonical.Path)
	if err != nil {
		return fmt.Errorf("failed to create parser: %w", err)
	}

	parsed, err := p.Parse(data)
	if err != nil {
		return fmt.Errorf("failed to parse data: %w", err)
	}

	flattened := diff.Flatten(parsed)
	slog.Info("canonical flattened config", "config", flattened)

	// Process each target
	for _, targetCfg := range cfg.Targets {
		slog.Info("Checking target", "name", targetCfg.Name, "type", targetCfg.Type)

		var adapter target.Adapter
		switch targetCfg.Type {
		case config.TypeSSH:
			adapter = target.NewSSHAdapter(targetCfg)
		default:
			slog.Error("Unsupported target type", "type", targetCfg.Type, "name", targetCfg.Name)
			continue
		}

		// fetching the target data
		targetData, err := adapter.Fetch(ctx)
		if err != nil {
			slog.Error("Failed to fetch target data", "name", targetCfg.Name, "error", err)
			continue
		}

		// getting the appropriate parser for target path
		tp, err := parser.FromPath(targetCfg.Path)
		if err != nil {
			slog.Error("Failed to get target parser", "name", targetCfg.Name, "path", targetCfg.Path, "error", err)
			continue
		}

		targetParsed, err := tp.Parse(targetData)
		if err != nil {
			slog.Error("Failed to parse target data", "name", targetCfg.Name, "error", err)
			continue
		}

		targetFlattened := diff.Flatten(targetParsed)
		slog.Info("target flattened config", "name", targetCfg.Name, "config", targetFlattened)

		result := diff.Compare(targetCfg.Name, flattened, targetFlattened)

		slog.Info("Drift Result", "target", targetCfg.Name, "is_drifted", result.IsDrifted)
		if result.IsDrifted {
			for _, d := range result.Added {
				slog.Warn("ADDED", "target", targetCfg.Name, "key", d.Key, "value", d.TargetValue)
			}
			for _, d := range result.Removed {
				slog.Warn("REMOVED", "target", targetCfg.Name, "key", d.Key, "value", d.CanonicalValue)
			}
			for _, d := range result.Changed {
				slog.Warn("CHANGED", "target", targetCfg.Name, "key", d.Key, "from", d.CanonicalValue, "to", d.TargetValue)
			}
		} else {
			slog.Info("No drift detected", "target", targetCfg.Name)
		}
	}

	return nil
}
