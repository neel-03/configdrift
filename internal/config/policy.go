package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/neel-03/configdrift/internal/utils"
)

// Supported canonical types
const (
	TypeGit   = "git"
	TypeLocal = "local"
	TypeS3    = "s3"
)

// Supported target types
const (
	TypeSSH    = "ssh"
	TypeDocker = "docker"
)

// CanonicalConfig represents the source of truth configuration.
type CanonicalConfig struct {
	Type      string `yaml:"type" validate:"required,oneof=git local s3"`
	Path      string `yaml:"path" validate:"required"`
	Repo      string `yaml:"repo" validate:"required_if=Type git"`
	Branch    string `yaml:"branch" validate:"required_if=Type git"`
	AuthToken string `yaml:"auth_token" validate:"omitempty,required_if=Type git"`
	S3Bucket  string `yaml:"bucket" validate:"required_if=Type s3"`
	S3Region  string `yaml:"region" validate:"required_if=Type s3"`
}

// Policy defines the drift detection settings.
type Policy struct {
	Canonical CanonicalConfig `yaml:"canonical" validate:"required"`
	Interval  string          `yaml:"interval" validate:"required"`
	Targets   []TargetConfig  `yaml:"targets" validate:"required,min=1,dive"`
}

// TargetConfig represents a remote config target.
type TargetConfig struct {
	Name       string `yaml:"name" validate:"required"`
	Type       string `yaml:"type" validate:"required,oneof=ssh docker"`
	Host       string `yaml:"host" validate:"required_if=Type ssh required_if=Type docker"`
	Port       int    `yaml:"port" validate:"omitempty"`
	User       string `yaml:"user" validate:"required_if=Type ssh"`
	Key        string `yaml:"key" validate:"required_if=Type ssh"`
	KnownHosts string `yaml:"known_hosts" validate:"omitempty"`
	Container  string `yaml:"container" validate:"required_if=Type docker"`
	Path       string `yaml:"path" validate:"required"`
	Timeout    string `yaml:"timeout" validate:"omitempty"`
}

// Load reads and validates a Policy from a YAML file.
func Load(path string) (p *Policy, err error) {
	root, err := os.OpenRoot(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open root directory: %w", err)
	}
	defer func() {
		if cerr := root.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close root directory: %w", cerr)
		}
	}()

	file, err := root.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open policy file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close policy file: %w", cerr)
		}
	}()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy file: %w", err)
	}

	// expand environment variables
	expanded := os.ExpandEnv(string(data))

	var policy Policy
	if err := utils.ParseYaml([]byte(expanded), &policy); err != nil {
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
