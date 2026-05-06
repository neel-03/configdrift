// Package logger provides logging utilities.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
)

// Init initializes the default slog logger to write in JSON format
// to both stdout and logs/all.log. It returns a cleanup function
// to close the log file.
func Init() (func(), error) {
	// make sure directory exists
	if err := os.MkdirAll("logs", 0750); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// open log file
	logFile, err := os.OpenFile("logs/all.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// using multiwriter for both stdout and file
	mw := io.MultiWriter(os.Stdout, logFile)

	// initializing slog with a JSON handler
	l := slog.New(slog.NewJSONHandler(mw, nil))
	slog.SetDefault(l)

	return func() {
		if err := logFile.Close(); err != nil {
			fmt.Printf("failed to close log file: %v\n", err)
		}
	}, nil
}
