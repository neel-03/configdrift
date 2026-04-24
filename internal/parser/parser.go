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
// Allowed extensions: .yaml, .yml
func FromPath(path string) (Parser, error) {
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case YAMLExt, YMLExt:
		return NewYAMLParser(), nil
	case TOMLext:
		return NewTOMLParser(), nil
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}
