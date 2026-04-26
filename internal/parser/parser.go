package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Parser converts raw bytes into structured map
type Parser interface {
	// Parse function should take raw bytes and return
	// a structured map or an error if parsing fails.
	Parse(data []byte) (map[string]interface{}, error)
}

// FromPath returns a parser instance based on the file extension.
// Supported extensions are registered using Register() at init time.
func FromPath(path string) (Parser, error) {
	ext := normalizeExt(filepath.Ext(path))
	if factory, ok := getParserFactory(ext); ok {
		return factory(), nil
	}

	// flexible env file detection fallback:
	// matches if 'env' is a dot-separated component
	// (e.g., .env, abc.env.xyz, env.pqr, my.env, env)
	base := strings.ToLower(filepath.Base(path))
	parts := strings.Split(base, ".")
	for _, part := range parts {
		if part == "env" {
			if factory, ok := getParserFactory(".env"); ok {
				return factory(), nil
			}
		}
	}

	return nil, fmt.Errorf("no parser registered for extension: %s. Supported: %v", ext, RegisteredExtensions())
}
