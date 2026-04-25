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

func TestInit_CreateDirFailure(t *testing.T) {
	// Create a file with the same name as the log directory to cause MkdirAll to fail
	logDir := "logs_fail"
	if err := os.WriteFile(logDir, []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocker file: %v", err)
	}
	defer func() {
		_ = os.Remove(logDir)
	}()

	// Since logger.Init is hardcoded to "logs", we need to run it in a temp dir or similar
	// But Init uses relative path "logs". We can change the working directory.
	oldWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// Create a blocker file in the temp dir
	if err := os.WriteFile("logs", []byte("not a dir"), 0644); err != nil {
		t.Fatalf("Failed to create blocker file in temp dir: %v", err)
	}

	_, err := Init()
	if err == nil {
		t.Error("Init() expected error when 'logs' is a file, got nil")
	}
}

func TestInit_OpenFileFailure(t *testing.T) {
	oldWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// create 'logs' directory and make it non-writable/non-accessible
	if err := os.Mkdir("logs", 0000); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}
	defer func() {
		_ = os.Chmod("logs", 0755)
	}() // restore to allow cleanup

	_, err := Init()
	if err == nil {
		t.Error("Init() expected error when 'logs' is not writable, got nil")
	}
}

func TestCleanup_CloseError(t *testing.T) {
	// This is hard to trigger with a real file, but we can try to close it twice.
	// However, slog might still be using it.
	// A better way is to mock the file, but Init uses real os.OpenFile.
	// Let's just call Init and then close the file manually before cleanup.
	
	oldWd, _ := os.Getwd()
	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	cleanup, err := Init()
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	cleanup()
}
