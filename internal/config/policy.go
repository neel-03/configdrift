package config

import (
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// Supported canonical types
const (
	TypeLocal = "local"
)

// CanonicalConfig represents the source of truth configuration.
type CanonicalConfig struct {
	Type string `yaml:"type" validate:"required,oneof=local"`
	Path string `yaml:"path" validate:"required"`
}

// Policy defines the drift detection settings.
type Policy struct {
	Canonical CanonicalConfig `yaml:"canonical" validate:"required"`
	Interval  string          `yaml:"interval" validate:"required"`
}

// Load reads and validates a Policy from a YAML file.
func Load(path string) (*Policy, error) {
	root, err := os.OpenRoot(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory: %w", err)
	}
	defer root.Close()

	file, err := root.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open policy file: %w", err)
	}
	defer file.Close()

	var policy Policy
	if err := yaml.NewDecoder(file).Decode(&policy); err != nil {
		return nil, fmt.Errorf("failed to decode policy YAML: %w", err)
	}

	v := validator.New()
	if err := v.Struct(&policy); err != nil {
		return nil, fmt.Errorf("policy validation failed: %w", err)
	}

	// interval should be valid duration
	if _, err := time.ParseDuration(policy.Interval); err != nil {
		return nil, fmt.Errorf("invalid interval duration %q: %w", policy.Interval, err)
	}

	return &policy, nil
}
