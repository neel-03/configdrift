package source

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func setupMockRepo(t *testing.T, branch, filename, content string) string {
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	w, err := repo.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = w.Add(filename)
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	hash, err := w.Commit("initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Create and checkout the target branch
	headRef := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branch), hash)
	err = repo.Storer.SetReference(headRef)
	if err != nil {
		t.Fatalf("failed to set reference: %v", err)
	}

	err = w.Checkout(&git.CheckoutOptions{
		Branch: headRef.Name(),
	})
	if err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}

	return dir
}

func updateMockRepo(t *testing.T, dir, filename, content string) {
	w, err := git.PlainOpen(dir)
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	wt, err := w.Worktree()
	if err != nil {
		t.Fatalf("failed to get worktree: %v", err)
	}

	fullPath := filepath.Join(dir, filename)
	if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	_, err = wt.Add(filename)
	if err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	_, err = wt.Commit("update file", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		t.Fatalf("failed to commit update: %v", err)
	}
}

// helper to create a GitSource with a unique cache for testing
func newTestGitSource(t *testing.T, repo, branch, file string) *GitSource {
	// Use a unique ID based on the test name and timestamp
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s|%d", t.Name(), time.Now().UnixNano())
	id := fmt.Sprintf("%x", hash.Sum(nil))[:12]

	cacheDir := filepath.Join(t.TempDir(), "configdrift", "git", id)

	return &GitSource{
		repoURL: repo,
		branch:  branch,
		file:    file,
		dir:     cacheDir,
	}
}

func TestGitSource_Fetch(t *testing.T) {
	branch := "main"
	filename := "config.yaml"
	content := "key: value"
	repoPath := setupMockRepo(t, branch, filename, content)

	t.Run("Public Repository without Token", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := gs.Fetch(ctx)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		if string(data) != content && string(data) != "key: updated_value" {
			t.Errorf("Expected content %q or %q, got %q", content, "key: updated_value", string(data))
		}
	})

	t.Run("Pull and Fetch Updated Content", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// first fetch to clone
		_, _ = gs.Fetch(ctx)

		newContent := "key: updated_value"
		updateMockRepo(t, repoPath, filename, newContent)

		data, err := gs.Fetch(ctx)
		if err != nil {
			t.Fatalf("Fetch failed after update: %v", err)
		}

		if string(data) != newContent {
			t.Errorf("Expected content %q, got %q", newContent, string(data))
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		_, err := gs.Fetch(ctx)
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	})

	t.Run("File Not Found in Repo", func(t *testing.T) {
		gsMissing := newTestGitSource(t, repoPath, branch, "missing.yaml")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := gsMissing.Fetch(ctx)
		if err == nil {
			t.Error("Expected error for missing file, got nil")
		}
	})

	t.Run("Invalid Repo URL", func(t *testing.T) {
		gsInvalid := newTestGitSource(t, "/non/existent/path", branch, filename)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := gsInvalid.Fetch(ctx)
		if err == nil {
			t.Error("Expected error for invalid repo URL, got nil")
		}
	})

	t.Run("Concurrency Safety", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		var wg sync.WaitGroup
		numGoroutines := 5
		errors := make([]error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, errors[idx] = gs.Fetch(ctx)
			}(i)
		}
		wg.Wait()

		for i := 0; i < numGoroutines; i++ {
			if errors[i] != nil {
				t.Errorf("Goroutine %d failed: %v", i, errors[i])
			}
		}
	})

	t.Run("Recovery from Corrupted Cache", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		// Manually create an empty directory to simulate a failed clone or corruption
		if err := os.MkdirAll(gs.dir, 0750); err != nil {
			t.Fatalf("Failed to create empty dir: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := gs.Fetch(ctx)
		if err != nil {
			t.Fatalf("Fetch failed to recover from empty dir: %v", err)
		}

		// content from the mock repo (which might have been updated by previous subtests)
		if len(data) == 0 {
			t.Error("Expected non-empty content")
		}
	})

	t.Run("Fetch with Auth Token", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		gs.authToken = "ghp_mock_token"

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Although go-git ignores Auth for local filesystem URLs,
		// this verifies that the code path for Auth is safe and doesn't crash.
		data, err := gs.Fetch(ctx)
		if err != nil {
			t.Fatalf("Fetch with token failed: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected non-empty content")
		}
	})

	t.Run("Missing Branch", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, "non-existent-branch", filename)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		_, err := gs.Fetch(ctx)
		if err == nil {
			t.Error("Expected error for missing branch, got nil")
		}
	})
}

func TestGitSource_String(t *testing.T) {
	gs := NewGitSource("https://github.com/user/repo", "main", "config.yaml", "")
	expected := "git::https://github.com/user/repo@main/config.yaml"
	if gs.String() != expected {
		t.Errorf("Expected %q, got %q", expected, gs.String())
	}
}
