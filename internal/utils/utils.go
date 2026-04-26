package utils

import (
	"fmt"
	"strconv"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
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
