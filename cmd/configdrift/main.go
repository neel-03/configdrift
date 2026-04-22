package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/logger"
	"github.com/neel-03/configdrift/internal/source"
)

func main() {
	// initializing the custom logger and getting cleanup function
	closeLogger, err := logger.Init()
	if err != nil {
		fmt.Println("failed to initialize logger:", err)
		os.Exit(1)
	}
	defer closeLogger()

	// load .env file if it exists
	if err := godotenv.Load(); err != nil {
		slog.Error("Error loading .env file", "error", err)
	}

	slog.Info("Configdrift starting")

	cfg, err := config.Load("./source.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
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
			cfg.Canonical.S3Key,
			cfg.Canonical.S3Region,
		)
		if err != nil {
			slog.Error("failed to init s3 source", "error", err)
			os.Exit(1)
		}
	default:
		slog.Error("unsupported source type", "type", cfg.Canonical.Type)
		os.Exit(1)
	}

	// create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// fetch data from source
	data, err := src.Fetch(ctx)
	if err != nil {
		slog.Error("failed to fetch data", "error", err)
		os.Exit(1)
	}

	slog.Info("data fetched", "data", string(data))

}
