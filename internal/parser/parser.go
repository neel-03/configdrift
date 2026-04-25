package parser

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	YAMLExt = ".yaml"
	YMLExt  = ".yml"
	TOMLext = ".toml"
)

// Parser converts raw bytes into structured map
type Parser interface {
	// Parse function should take raw bytes and return
	// a structured map or an error if parsing fails.
	Parse(data []byte) (map[string]interface{}, error)
}

// FromPath returns a parser instance based on the file extension.
// Allowed extensions: .yaml, .yml, .toml, .env (and variations like abc.env.xyz)
func FromPath(path string) (Parser, error) {
	base := strings.ToLower(filepath.Base(path))
	ext := filepath.Ext(base)

	// checking for standard config extensions first
	switch ext {
	case YAMLExt, YMLExt:
		return NewYAMLParser(), nil
	case TOMLext:
		return NewTOMLParser(), nil
	}
	// flexible env file detection:
	// matches if 'env' is a dot-separated component
	// (e.g., .env, abc.env.xyz, env.pqr, my.env, env)
	parts := strings.Split(base, ".")
	for _, part := range parts {
		if part == "env" {
			return NewEnvParser(), nil
		}
	}

	return nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(path))
}
