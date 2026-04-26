package parser

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Factory is a function that creates a new Parser instance
type Factory func() Parser

var (
	mu       sync.RWMutex
	registry = make(map[string]Factory)
)

// RegisterParser adds a new parser factory to the registry for the given extension.
// It panics if the factory is nil, the extension is empty, or if the extension is already registered.
func RegisterParser(ext string, factory Factory) {
	if factory == nil {
		panic("parser: factory cannot be nil")
	}

	ext = normalizeExt(ext)
	if ext == "" || ext == "." {
		panic("parser: extension cannot be empty")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, dup := registry[ext]; dup {
		panic(fmt.Sprintf("parser: RegisterParser called twice for extension %s", ext))
	}

	registry[ext] = factory
}

// RegisteredExtensions returns a sorted list of all registered file extensions.
func RegisteredExtensions() []string {
	mu.RLock()
	defer mu.RUnlock()

	exts := make([]string, 0, len(registry))
	for ext := range registry {
		exts = append(exts, ext)
	}
	sort.Strings(exts)
	return exts
}

// getParserFactory retrieves a parser factory from the registry.
func getParserFactory(ext string) (Factory, bool) {
	mu.RLock()
	defer mu.RUnlock()

	f, ok := registry[ext]
	return f, ok
}

// normalizeExt ensures the extension is lowercase, trimmed, and has a leading dot
func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}
