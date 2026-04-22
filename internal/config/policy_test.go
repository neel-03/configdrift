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
		{
			name: "valid git policy without token (empty string)",
			yaml: `
canonical:
  type: git
  repo: https://github.com/user/repo
  branch: main
  path: config.yaml
  auth_token: ""
interval: 1m
`,
			wantErr: false,
		},
		{
			name: "valid git policy without token (field missing)",
			yaml: `
canonical:
  type: git
  repo: https://github.com/user/repo
  branch: main
  path: config.yaml
interval: 1m
`,
			wantErr: false,
		},
		{
			name: "valid git policy with token",
			yaml: `
canonical:
  type: git
  repo: https://github.com/user/repo
  branch: main
  path: config.yaml
  auth_token: my-secret-token
interval: 1m
`,
			wantErr: false,
		},
		{
			name: "git policy with env var token",
			yaml: `
canonical:
  type: git
  repo: https://github.com/user/repo
  branch: main
  path: config.yaml
  auth_token: ${TEST_TOKEN}
interval: 1m
`,
			wantErr: false,
		},
		{
			name: "git policy missing repo",
			yaml: `
canonical:
  type: git
  branch: main
  path: config.yaml
interval: 1m
`,
			wantErr: true,
		},
		{
			name: "git policy missing branch",
			yaml: `
canonical:
  type: git
  repo: https://github.com/user/repo
  path: config.yaml
interval: 1m
`,
			wantErr: true,
		},
	}

	os.Setenv("TEST_TOKEN", "env-secret-token")
	defer os.Unsetenv("TEST_TOKEN")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpfile, err := os.CreateTemp(".", "policy-*.yaml")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(tmpfile.Name()); err != nil {
					t.Errorf("Failed to remove temporary file: %v", err)
				}
			}()

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
