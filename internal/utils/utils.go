package utils

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// ParseYaml unmarshals raw YAML bytes into the provided target structure.
func ParseYaml(data []byte, target interface{}) error {
	if err := yaml.Unmarshal(data, target); err != nil {
		return fmt.Errorf("yaml parse failed: %w", err)
	}
	return nil
}
