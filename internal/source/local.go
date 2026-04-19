package source

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LocalSource reads the canonical config from a local file path.
type LocalSource struct {
	path        string
	mu          sync.RWMutex
	lastModTime time.Time
	cachedData  []byte
}

// NewLocalSource creates a new local source.
func NewLocalSource(path string) *LocalSource {
	return &LocalSource{
		path: filepath.Clean(path),
	}
}

// Fetch reads the canonical config from a local file path and returns the bytes.
func (ls *LocalSource) Fetch(ctx context.Context) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	info, err := os.Stat(ls.path)
	if err != nil {
		return nil, fmt.Errorf("local source stat %s: %w", ls.path, err)
	}

	// Double-checked locking pattern for efficiency
	ls.mu.RLock()
	if info.ModTime().Equal(ls.lastModTime) && ls.cachedData != nil {
		data := make([]byte, len(ls.cachedData))
		copy(data, ls.cachedData)
		ls.mu.RUnlock()
		return data, nil
	}
	ls.mu.RUnlock()

	ls.mu.Lock()
	defer ls.mu.Unlock()

	// Re-check after acquiring write lock
	if info.ModTime().Equal(ls.lastModTime) && ls.cachedData != nil {
		data := make([]byte, len(ls.cachedData))
		copy(data, ls.cachedData)
		return data, nil
	}

	data, err := os.ReadFile(ls.path) // #nosec G304
	if err != nil {
		return nil, fmt.Errorf("local source read %s: %w", ls.path, err)
	}

	ls.cachedData = data
	ls.lastModTime = info.ModTime()

	// Return a copy to avoid external mutation of cache
	res := make([]byte, len(data))
	copy(res, data)
	return res, nil
}

// String returns the cleaned path of the local source.
func (ls *LocalSource) String() string {
	return ls.path
}
