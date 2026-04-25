package main

import (
	"os"
	"testing"
)

func TestRun(t *testing.T) {
	// Setup a temporary source.yaml and a config file
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// Create a dummy config file to be fetched by local source
	configData := "KEY=VALUE"
	if err := os.WriteFile("local.env", []byte(configData), 0644); err != nil {
		t.Fatal(err)
	}

	sourceYaml := `
canonical:
  type: local
  path: ./local.env
interval: 1m
`
	if err := os.WriteFile("source.yaml", []byte(sourceYaml), 0644); err != nil {
		t.Fatal(err)
	}

	// We need to handle the logs directory because Init() creates it
	defer func() {
		_ = os.RemoveAll("logs")
	}()

	err = run()
	if err != nil {
		t.Errorf("run() failed: %v", err)
	}
}

func TestRun_ConfigError(t *testing.T) {
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	// but currently it's hardcoded to ./source.yaml.
	// Since we are in a temp dir and didn't create source.yaml here:
	err = run()
	if err == nil {
		t.Error("run() expected error for non-existent config, got nil")
	}
}
