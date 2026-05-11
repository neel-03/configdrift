// Package utils provides utility functions.
package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// GenerateID returns a random 64-character hex string.
func GenerateID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// ParseYaml unmarshals raw YAML bytes into the provided target structure.
func ParseYaml(data []byte, target interface{}) error {
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("yaml parse failed: %w", err)
	}
	return nil
}

// ParseToml unmarshals raw TOML bytes into the provided target structure.
func ParseToml(data []byte, target interface{}) error {
	if err := toml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("toml parse failed: %w", err)
	}
	return nil
}

// ParseEnv parses .env style data into the provided target map
func ParseEnv(data []byte, target interface{}) error {
	envMap, err := godotenv.Unmarshal(string(data))
	if err != nil {
		return fmt.Errorf("env parse failed: %w", err)
	}

	if m, ok := target.(*map[string]interface{}); ok {
		if *m == nil {
			*m = make(map[string]interface{})
		}
		for k, v := range envMap {
			(*m)[k] = v
		}
		return nil
	}

	return fmt.Errorf("unsupported target type for env parsing: %T", target)
}

// IndexToKey converts an integer index to a string key suitable for dot notation
// e.g.
// 0 -> "[0]", 1 -> "[1]" ...
func IndexToKey(i int) string {
	return "[" + strconv.Itoa(i) + "]"
}

// ExpandPath resolves ~ in paths as os.ReadFile
// won't expand the tilde automatically.
func ExpandPath(path string) string {
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path // can't expand, we return as-is, let the caller handle the error
	}
	return filepath.Join(home, path[1:])
}
