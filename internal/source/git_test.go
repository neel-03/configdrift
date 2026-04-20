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
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
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
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
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
	hash.Write([]byte(fmt.Sprintf("%s|%d", t.Name(), time.Now().UnixNano())))
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

	t.Run("Initial Clone and Fetch", func(t *testing.T) {
		gs := newTestGitSource(t, repoPath, branch, filename)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		data, err := gs.Fetch(ctx)
		if err != nil {
			t.Fatalf("Fetch failed: %v", err)
		}

		if string(data) != content {
			t.Errorf("Expected content %q, got %q", content, string(data))
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
}

func TestGitSource_String(t *testing.T) {
	gs := NewGitSource("https://github.com/user/repo", "main", "config.yaml")
	expected := "git::https://github.com/user/repo@main/config.yaml"
	if gs.String() != expected {
		t.Errorf("Expected %q, got %q", expected, gs.String())
	}
}
