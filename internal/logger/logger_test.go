package logger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	// Clean up any existing logs directory to test creation
	logDir := "logs"
	logFile := filepath.Join(logDir, "all.log")
	
	// Ensure we are in a clean state (relative to test execution path)
	os.RemoveAll(logDir)
	defer os.RemoveAll(logDir)

	cleanup, err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}
	
	if cleanup == nil {
		t.Fatal("Init() returned nil cleanup function")
	}
	defer cleanup()

	// Check if directory was created
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		t.Errorf("Init() did not create %s directory", logDir)
	}

	// Check if log file was created
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Errorf("Init() did not create %s file", logFile)
	}
}

func TestInit_DirPermissions(t *testing.T) {
	// This test depends on the environment and might be flaky on some systems,
	// but it tests if Init handles existing directories.
	logDir := "logs"
	os.MkdirAll(logDir, 0755)
	defer os.RemoveAll(logDir)

	cleanup, err := Init()
	if err != nil {
		t.Errorf("Init() failed with existing directory: %v", err)
	}
	if cleanup != nil {
		cleanup()
	}
}
