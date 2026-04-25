package parser

import (
	"testing"
)

func TestFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string // "yaml", "toml", "env", or "error"
	}{
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"config.toml", "toml"},
		{".env", "env"},
		{"abc.env.xyz", "env"},
		{"env.pqr", "env"},
		{"my.env", "env"},
		{"env", "env"},
		{"production.env.local", "env"},
		{"other.txt", "error"},
		{"config.yaml.env", "env"}, // Should be env because .yaml is not the last extension, and 'env' is a component
		{"config.env.yaml", "yaml"}, // Should be yaml because .yaml is the last extension and we check standard extensions first
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			p, err := FromPath(tt.path)
			if tt.expected == "error" {
				if err == nil {
					t.Errorf("FromPath(%s) expected error, got nil", tt.path)
				}
				return
			}

			if err != nil {
				t.Errorf("FromPath(%s) unexpected error: %v", tt.path, err)
				return
			}

			var actual string
			switch p.(type) {
			case *YAMLParser:
				actual = "yaml"
			case *TOMLParser:
				actual = "toml"
			case *EnvParser:
				actual = "env"
			default:
				actual = "unknown"
			}

			if actual != tt.expected {
				t.Errorf("FromPath(%s) expected %s, got %s", tt.path, tt.expected, actual)
			}
		})
	}
}
