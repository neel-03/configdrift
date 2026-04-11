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
	if err := os.RemoveAll(logDir); err != nil {
		t.Fatalf("Failed to clean up before test: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(logDir); err != nil {
			t.Errorf("Failed to clean up after test: %v", err)
		}
	}()

	cleanup, err := Init()
	if err != nil {
		t.Fatalf("Init() failed: %v", err)
	}

	if cleanup == nil {
		t.Fatal("Init() returned nil cleanup function")
	}
	defer cleanup()

	// Check if directory was created
	if info, err := os.Stat(logDir); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Init() did not create %s directory", logDir)
		} else {
			t.Errorf("Failed to stat directory: %v", err)
		}
	} else if !info.IsDir() {
		t.Errorf("%s is not a directory", logDir)
	}

	// Check if log file was created
	if _, err := os.Stat(logFile); err != nil {
		if os.IsNotExist(err) {
			t.Errorf("Init() did not create %s file", logFile)
		} else {
			t.Errorf("Failed to stat log file: %v", err)
		}
	}
}

func TestInit_DirPermissions(t *testing.T) {
	// This test depends on the environment and might be flaky on some systems,
	// but it tests if Init handles existing directories.
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0750); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(logDir); err != nil {
			t.Errorf("Failed to clean up: %v", err)
		}
	}()

	cleanup, err := Init()
	if err != nil {
		t.Errorf("Init() failed with existing directory: %v", err)
	}
	if cleanup != nil {
		cleanup()
	}
}
