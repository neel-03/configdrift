package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid local policy",
			yaml: `
canonical:
  type: local
  path: ./config.yaml
interval: 1m
`,
			wantErr: false,
		},
		{
			name: "missing canonical block",
			yaml: `
interval: 1m
`,
			wantErr: true,
		},
		{
			name: "missing type",
			yaml: `
canonical:
  path: ./config.yaml
interval: 1m
`,
			wantErr: true,
		},
		{
			name: "invalid type (unknown)",
			yaml: `
canonical:
  type: remote
  path: ./config.yaml
interval: 1m
`,
			wantErr: true,
		},
		{
			name: "missing local path",
			yaml: `
canonical:
  type: local
interval: 1m
`,
			wantErr: true,
		},
		{
			name: "empty interval",
			yaml: `
canonical:
  type: local
  path: ./config.yaml
interval: ""
`,
			wantErr: true,
		},
		{
			name: "invalid interval format (non-duration)",
			yaml: `
canonical:
  type: local
  path: ./config.yaml
interval: "2 hours"
`,
			wantErr: true,
		},
		{
			name: "invalid interval unit",
			yaml: `
canonical:
  type: local
  path: ./config.yaml
interval: 10x
`,
			wantErr: true,
		},
		{
			name: "garbage content",
			yaml: `
!!! this is not yaml !!!
`,
			wantErr: true,
		},
		{
			name:    "empty file",
			yaml:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp("", "policy-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpfile.Name())

			if _, err := tmpfile.Write([]byte(tt.yaml)); err != nil {
				t.Fatal(err)
			}
			if err := tmpfile.Close(); err != nil {
				t.Fatal(err)
			}

			_, err = Load(tmpfile.Name())
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	t.Run("file not found", func(t *testing.T) {
		_, err := Load("non-existent-file.yaml")
		if err == nil {
			t.Errorf("Load() expected error for non-existent file, got nil")
		}
	})
}
