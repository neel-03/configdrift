package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/neel-03/configdrift/internal/config"
	"github.com/neel-03/configdrift/internal/logger"
)

func main() {
	// initializing the custom logger and getting cleanup function
	closeLogger, err := logger.Init()
	if err != nil {
		fmt.Println("failed to initialize logger:", err)
		os.Exit(1)
	}
	defer closeLogger()

	slog.Info("Configdrift starting")

	cfg, err := config.Load("./targets.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("config loaded", "config", cfg)
}
