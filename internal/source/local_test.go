package source

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLocalSource(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yaml")
		content := []byte("key: value")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		src := NewLocalSource(path)
		data, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}
		if string(data) != string(content) {
			t.Errorf("expected %s, got %s", content, data)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		src := NewLocalSource("non_existent_file_xyz.yaml")
		_, err := src.Fetch(context.Background())
		if err == nil {
			t.Fatal("expected error for non-existent file, got nil")
		}
	})

	t.Run("CachingAndUpdating", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yaml")

		content1 := []byte("v1")
		if err := os.WriteFile(path, content1, 0644); err != nil {
			t.Fatal(err)
		}

		src := NewLocalSource(path)

		// First fetch - should read from disk
		data1, err := src.Fetch(context.Background())
		if err != nil || string(data1) != "v1" {
			t.Fatalf("first fetch failed: %v, data: %s", err, data1)
		}

		// Update file and ensure ModTime changes
		content2 := []byte("v2")
		if err := os.WriteFile(path, content2, 0644); err != nil {
			t.Fatal(err)
		}

		// Explicitly change ModTime to ensure the cache is invalidated
		newTime := time.Now().Add(1 * time.Hour)
		if err := os.Chtimes(path, newTime, newTime); err != nil {
			t.Fatal(err)
		}

		data2, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatalf("second fetch failed: %v", err)
		}
		if string(data2) != "v2" {
			t.Errorf("expected v2 after modtime change, got %s", data2)
		}

		// Fetch again without change - should be fast and same
		data3, err := src.Fetch(context.Background())
		if err != nil || string(data3) != "v2" {
			t.Fatalf("third fetch failed: %v", err)
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		src := NewLocalSource("any.path")
		_, err := src.Fetch(ctx)
		if err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Concurrency", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yaml")
		content := []byte("concurrent data")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		src := NewLocalSource(path)
		var wg sync.WaitGroup
		numOroutines := 50

		for i := 0; i < numOroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				data, err := src.Fetch(context.Background())
				if err != nil || string(data) != string(content) {
					t.Errorf("concurrent fetch failed or data mismatch")
				}
			}()
		}
		wg.Wait()
	})

	t.Run("DefensiveCopying", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "config.yaml")
		content := []byte("original")
		if err := os.WriteFile(path, content, 0644); err != nil {
			t.Fatal(err)
		}

		src := NewLocalSource(path)

		// Fetch and modify the slice
		data1, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		data1[0] = 'X'

		// Fetch again - should still be original
		data2, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if string(data2) != "original" {
			t.Errorf("internal cache was mutated! expected 'original', got %s", data2)
		}
	})

	t.Run("EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "empty.yaml")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		src := NewLocalSource(path)
		data, err := src.Fetch(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty data, got %v", data)
		}
	})

	t.Run("StringMethod", func(t *testing.T) {
		path := "/tmp/test.yaml"
		src := NewLocalSource(path)
		if src.String() != filepath.Clean(path) {
			t.Errorf("String() returned %s, expected %s", src.String(), filepath.Clean(path))
		}
	})
}
