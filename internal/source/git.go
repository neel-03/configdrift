package source

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GitSource fetches the canonical config from a remote git repository.
type GitSource struct {
	repoURL string
	branch  string
	file    string
	dir     string // local cache directory
	mu      sync.Mutex
}

// NewGitSource creates a new git source.
func NewGitSource(repo, branch, file string) *GitSource {
	cacheDir := buildCacheDir(repo, branch)

	return &GitSource{
		repoURL: repo,
		branch:  branch,
		file:    file,
		dir:     cacheDir,
	}
}

// Fetch reads the canonical config from a remote git repository and returns the bytes.
func (gs *GitSource) Fetch(ctx context.Context) ([]byte, error) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// ensure repo exists and is up to date
	if _, err := gs.ensureRepo(); err != nil {
		return nil, err
	}

	// read the file
	root, err := os.OpenRoot(gs.dir)
	if err != nil {
		return nil, fmt.Errorf("open root %s: %w", gs.dir, err)
	}
	defer root.Close()

	f, err := root.Open(gs.file)
	if err != nil {
		return nil, fmt.Errorf("open git source file %s: %w", gs.file, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read git source file %s: %w", gs.file, err)
	}

	return data, nil
}

// String returns a human-readable identifier for the git source.
func (gs *GitSource) String() string {
	return fmt.Sprintf("git::%s@%s/%s", gs.repoURL, gs.branch, gs.file)
}

// buildCacheDir builds a unique cache directory for the given repository and branch.
func buildCacheDir(repo, branch string) string {
	// Use SHA256 to avoid path collisions and keep directory names safe
	hash := sha256.New()
	hash.Write([]byte(fmt.Sprintf("%s|%s", repo, branch)))
	id := fmt.Sprintf("%x", hash.Sum(nil))[:12]

	baseDir, err := os.UserCacheDir()
	if err != nil {
		baseDir = os.TempDir()
	}

	return filepath.Join(baseDir, "configdrift", "git", id)
}

func (gs *GitSource) ensureRepo() (*git.Repository, error) {
	if _, err := os.Stat(gs.dir); os.IsNotExist(err) {
		// if repo doesn't exist locally, clone it
		return gs.clone()
	}
	// if repo exists, pull latest changes
	return gs.pull()
}

// clone the repo if it doesn't exist locally
func (gs *GitSource) clone() (*git.Repository, error) {
	if err := os.MkdirAll(gs.dir, 0750); err != nil {
		return nil, fmt.Errorf("create repo dir: %w", err)
	}

	repo, err := git.PlainClone(
		gs.dir,
		false,
		&git.CloneOptions{
			URL:           gs.repoURL,
			ReferenceName: plumbing.NewBranchReferenceName(gs.branch),
			SingleBranch:  true,
			Depth:         1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("clone repo: %w", err)
	}

	return repo, nil
}

// pull latest changes from remote repo
func (gs *GitSource) pull() (*git.Repository, error) {
	repo, err := git.PlainOpen(gs.dir)
	if err != nil {
		return nil, fmt.Errorf("open repo: %w", err)
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, fmt.Errorf("get worktree: %w", err)
	}

	err = tree.Pull(&git.PullOptions{
		RemoteName: "origin",
	})
	if err != nil && err != git.NoErrAlreadyUpToDate {
		// If pull fails (e.g. history diverged), we could potentially
		// delete and re-clone, but for now we report it as an error.
		return nil, fmt.Errorf("pull repo: %w", err)
	}

	return repo, nil
}
