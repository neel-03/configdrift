package utils

import (
	"fmt"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

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
